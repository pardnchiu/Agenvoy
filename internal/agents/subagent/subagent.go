package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	toolName       = "invoke_subagent"
	defaultTimeout = 10 * time.Minute
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		ReadOnly:   true,
		Concurrent: true,
		Name:       toolName,
		Description: `分派一個**內部**子 agent 處理子任務並回傳結果，用於工作流程拆解、平行委派與專長模型分工。子 agent 走本專案 exec 引擎，共用 model registry 與所有本專案 tool（檔案、搜尋、git 等），擁有獨立 session 與 context，與主 agent 完全隔離；僅回傳最終文字結果，主 agent 自行整合。子 agent 預設禁止再次呼叫 invoke_subagent，避免無限巢狀。

與 invoke_external_agent 的差別：本 tool 是**內部**委派（共用本專案 tool 與 model registry）；invoke_external_agent 是呼叫外部 CLI（codex／copilot／claude），外部 agent 無法存取本專案 tool。需要本專案 tool 協助完成的任務一律用 invoke_subagent。`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "子 agent 要處理的完整任務描述（自包含，子 agent 看不到主 agent 的對話歷史）",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "（可選）指定 worker model 名稱，必須是已註冊的 model；留空則由 planner 自動挑選",
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "（可選）追加給子 agent 的角色／限制；會插入到 system prompt 的 Additional Instructions 區塊",
				},
				"exclude_tools": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "（可選）額外排除的 tool 名稱清單；invoke_subagent 自身一律強制排除",
				},
			},
			"required": []string{"task"},
		},
		Handler: handle,
	})
}

func handle(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
	var params struct {
		Task         string   `json:"task"`
		Model        string   `json:"model,omitempty"`
		SystemPrompt string   `json:"system_prompt,omitempty"`
		ExcludeTools []string `json:"exclude_tools,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	task := strings.TrimSpace(params.Task)
	if task == "" {
		return "", fmt.Errorf("task is required")
	}

	registry := host.Registry()
	planner := host.Planner()
	if planner == nil || len(registry.Registry) == 0 {
		return "", fmt.Errorf("subagent host not initialized")
	}

	var agent agentTypes.Agent
	if params.Model != "" {
		if a, ok := registry.Registry[params.Model]; ok {
			agent = a
		} else {
			return "", fmt.Errorf("model not found: %s", params.Model)
		}
	} else {
		agent = exec.SelectAgent(ctx, planner, registry, task, false)
	}
	if agent == nil {
		return "", fmt.Errorf("no agent available")
	}

	sessionID, err := sessionManager.CreateSession("temp-sub-")
	if err != nil {
		return "", fmt.Errorf("CreateSession: %w", err)
	}

	excluded := append([]string{toolName}, params.ExcludeTools...)
	execData := exec.ExecData{
		Agent:             agent,
		WorkDir:           ".",
		Content:           task,
		ExcludeTools:      excluded,
		ExtraSystemPrompt: params.SystemPrompt,
	}

	userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), task)
	session := &agentTypes.AgentSession{
		ID:            sessionID,
		SystemPrompts: []agentTypes.Message{{Role: "system", Content: exec.GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, host.Scanner())}},
		OldHistories:  []agentTypes.Message{},
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		Histories:     []agentTypes.Message{{Role: "user", Content: userText}},
		UserInput:     agentTypes.Message{Role: "user", Content: userText},
	}

	subCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	events := make(chan agentTypes.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- exec.Execute(subCtx, execData, session, events, true)
		close(events)
	}()

	var sb strings.Builder
	for ev := range events {
		switch ev.Type {
		case agentTypes.EventText:
			if ev.Text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(ev.Text)
		case agentTypes.EventToolConfirm:
			if ev.ReplyCh != nil {
				ev.ReplyCh <- true
			}
		case agentTypes.EventError:
			if ev.Err != nil {
				slog.Warn("subagent event error",
					slog.String("session", sessionID),
					slog.String("error", ev.Err.Error()))
			}
		}
	}

	if err := <-errCh; err != nil {
		text := strings.TrimSpace(sb.String())
		if text == "" {
			return "", fmt.Errorf("subagent execute: %w", err)
		}
		return fmt.Sprintf("[subagent partial result · %s]\n%s\n\n[error] %s",
			agent.Name(), text, err.Error()), nil
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return fmt.Sprintf("[subagent · %s] 未產出文字結果", agent.Name()), nil
	}
	return fmt.Sprintf("[subagent · %s]\n%s", agent.Name(), result), nil
}

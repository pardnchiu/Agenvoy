package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
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
	defaultTimeout = 10 * time.Minute
)

func Register() {
	models := []string{}
	for _, m := range exec.GetAgent() {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}

	toolRegister.Regist(toolRegister.Def{
		Name:       "invoke_subagent",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Dispatch an **internal** subagent to handle a subtask and return the result,
useful for workflow decomposition, parallel delegation, and specialist model division of labor.

The subagent runs through this project's exec engine, sharing the model registry and all tools in this project (files, search, git, etc.).
It has an independent session and context, fully isolated from the main agent; it only returns the final text result, which the main agent integrates on its own.

By default, the subagent is prohibited from calling **invoke_subagent** again to avoid infinite nesting.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "The subagent's complete task description (self-contained; the subagent cannot see the main agent's conversation history)",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "(Optional) Specify the worker model name; it must be a registered model. Leave blank to let the planner choose automatically",
					"default":     "",
					"enum":        models,
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "(Optional) Additional role or constraints for the subagent; will be inserted into the 'Additional Instructions' section of the system prompt.",
					"default":     "",
				},
				"exclude_tools": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "(Optional) Additional list of tool names to exclude; invoke_subagent itself is always forcibly excluded",
					"default":     []string{},
				},
			},
			"required": []string{
				"task",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
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

			// avoid small agent like 4.1 be stupid to call with not support value
			model := strings.TrimSpace(params.Model)
			if model != "" && !slices.Contains(models, model) {
				slog.Warn("invalid model, fallback to auto-select")
				model = ""
			}

			systemPrompt := strings.TrimSpace(params.SystemPrompt)

			return Exec(ctx, task, model, systemPrompt, params.ExcludeTools)
		},
	})
}

func Exec(ctx context.Context, task, model, systemPrompt string, excludedTools []string) (string, error) {
	registry := host.Registry()
	planner := host.Planner()
	if planner == nil || len(registry.Registry) == 0 {
		return "", fmt.Errorf("subagent host not initialized")
	}

	var agent agentTypes.Agent
	if model != "" {
		agent = registry.Registry[model]
	} else {
		agent = exec.SelectAgent(ctx, planner, registry, task, false)
	}
	if agent == nil {
		return "", fmt.Errorf("no agent available")
	}

	sessionID, err := sessionManager.CreateSession("temp-sub-")
	if err != nil {
		return "", fmt.Errorf("sessionManager.CreateSession: %w", err)
	}

	excluded := append([]string{"invoke_subagent"}, excludedTools...)
	execData := exec.ExecData{
		Agent:             agent,
		WorkDir:           ".",
		Content:           task,
		ExcludeTools:      excluded,
		ExtraSystemPrompt: systemPrompt,
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

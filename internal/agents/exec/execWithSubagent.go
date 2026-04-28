package exec

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
	go_utils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

const (
	defaultSubagentTimeoutMin = 10
	hardCapSubagentTimeoutMin = 60
)

var SubagentTimeoutMin = max(defaultSubagentTimeoutMin,
	min(hardCapSubagentTimeoutMin,
		go_utils_utils.GetWithDefaultInt("MAX_SUBAGENT_TIMEOUT_MIN", defaultSubagentTimeoutMin)))

func ExecWithSubagent(ctx context.Context, task, sessionIDInput, model, systemPrompt string, excludedTools []string) (string, error) {
	registry := host.Registry()
	planner := host.Planner()
	if planner == nil || len(registry.Registry) == 0 {
		return "", fmt.Errorf("subagent host not initialized")
	}

	var agent agentTypes.Agent
	if model != "" {
		agent = registry.Registry[model]
	} else {
		agent = SelectAgent(ctx, planner, registry, task, false)
	}
	if agent == nil {
		return "", fmt.Errorf("no agent available")
	}

	sessionID, err := ensureSubagentSession(sessionIDInput)
	if err != nil {
		return "", fmt.Errorf("ensureSubagentSession: %w", err)
	}

	excluded := append([]string{"invoke_subagent", "invoke_external_agent", "cross_review_with_external_agents", "review_result", "ask_user"}, excludedTools...)
	execData := ExecData{
		Agent:             agent,
		WorkDir:           ".",
		Content:           task,
		ExcludeTools:      excluded,
		ExtraSystemPrompt: systemPrompt,
		AllowAll:          true,
	}

	oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
	if oldHistory == nil {
		oldHistory = []agentTypes.Message{}
	}
	if maxHistory == nil {
		maxHistory = []agentTypes.Message{}
	}

	userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), task)

	histories := append([]agentTypes.Message{}, oldHistory...)
	histories = append(histories, agentTypes.Message{Role: "user", Content: userText})

	session := &agentTypes.AgentSession{
		ID:            sessionID,
		SystemPrompts: []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, host.Scanner(), sessionID, execData.AllowAll)}},
		OldHistories:  maxHistory,
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		Histories:     histories,
		UserInput:     agentTypes.Message{Role: "user", Content: userText},
	}
	if summary := sessionManager.GetSummaryPrompt(sessionID, OldestMessageTime(maxHistory)); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	SaveUserInputHistory(sessionID, userText)

	subCtx, cancel := context.WithTimeout(ctx, time.Duration(SubagentTimeoutMin)*time.Minute)
	defer cancel()

	events := make(chan agentTypes.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(subCtx, execData, session, events, true)
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
		return fmt.Sprintf("[subagent partial result · %s · session=%s]\n%s\n\n[error] %s",
			agent.Name(), sessionID, text, err.Error()), nil
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return fmt.Sprintf("[subagent · %s · session=%s] 未產出文字結果", agent.Name(), sessionID), nil
	}
	return fmt.Sprintf("[subagent · %s · session=%s]\n%s", agent.Name(), sessionID, result), nil
}

func ensureSubagentSession(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		id, err := sessionManager.CreateSession("temp-sub-")
		if err != nil {
			return "", fmt.Errorf("sessionManager.CreateSession: %w", err)
		}
		return id, nil
	}

	sessionDir := filepath.Join(filesystem.SessionsDir, trimmed)
	if !go_utils_filesystem.Exists(sessionDir) {
		return "", fmt.Errorf("session %q does not exist", trimmed)
	}
	if !go_utils_filesystem.IsDir(sessionDir) {
		return "", fmt.Errorf("session %q is not a directory", trimmed)
	}
	return trimmed, nil
}

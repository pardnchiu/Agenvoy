package exec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func ExecWithSubagent(ctx context.Context, task, model, systemPrompt string, excludedTools []string) (string, error) {
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

	sessionID, err := sessionManager.CreateSession("temp-sub-")
	if err != nil {
		return "", fmt.Errorf("sessionManager.CreateSession: %w", err)
	}

	excluded := append([]string{"invoke_subagent", "invoke_external_agent", "cross_review_with_external_agents", "review_result"}, excludedTools...)
	execData := ExecData{
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
		SystemPrompts: []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, host.Scanner())}},
		OldHistories:  []agentTypes.Message{},
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		Histories:     []agentTypes.Message{{Role: "user", Content: userText}},
		UserInput:     agentTypes.Message{Role: "user", Content: userText},
	}

	subCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
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
		return fmt.Sprintf("[subagent partial result · %s]\n%s\n\n[error] %s",
			agent.Name(), text, err.Error()), nil
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return fmt.Sprintf("[subagent · %s] 未產出文字結果", agent.Name()), nil
	}
	return fmt.Sprintf("[subagent · %s]\n%s", agent.Name(), result), nil
}

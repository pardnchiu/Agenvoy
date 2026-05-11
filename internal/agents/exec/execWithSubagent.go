package exec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

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
		go_pkg_utils.GetWithDefaultInt("MAX_SUBAGENT_TIMEOUT_MIN", defaultSubagentTimeoutMin)))

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
		agent = SelectAgent(ctx, planner, registry, task, false, sessionIDInput)
	}
	if agent == nil {
		return "", fmt.Errorf("no agent available")
	}

	sessionID, err := ensureSubagentSession(sessionIDInput)
	if err != nil {
		return "", fmt.Errorf("ensureSubagentSession: %w", err)
	}

	allowAll, ok := ctx.Value(allowAllCtxKey{}).(bool)
	if !ok {
		allowAll = true
	}

	workDir, ok := ctx.Value(parentWorkDirKey{}).(string)
	if !ok || workDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			workDir = cwd
		} else if home, err := os.UserHomeDir(); err == nil {
			workDir = home
		} else {
			return "", fmt.Errorf("cwd and home both failed")
		}
	}
	excluded := append([]string{"invoke_subagent", "invoke_external_agent", "cross_review_with_external_agents", "review_result"}, excludedTools...)
	execData := ExecData{
		Agent:             agent,
		WorkDir:           workDir,
		Content:           task,
		ExcludeTools:      excluded,
		ExtraSystemPrompt: systemPrompt,
		AllowAll:          allowAll,
	}

	oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
	if oldHistory == nil {
		oldHistory = []agentTypes.Message{}
	}
	if maxHistory == nil {
		maxHistory = []agentTypes.Message{}
	}

	userText := fmt.Sprintf("---\n當前時間: %s\n工作目錄: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), execData.WorkDir, task)

	histories := append([]agentTypes.Message{}, oldHistory...)
	histories = append(histories, agentTypes.Message{Role: "user", Content: userText})

	session := &agentTypes.AgentSession{
		ID:            sessionID,
		SystemPrompts: []agentTypes.Message{{Role: "system", Content: GetSystemPrompt(execData.WorkDir, execData.ExtraSystemPrompt, host.Scanner(), sessionID, execData.AllowAll, false)}},
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

	parentEvents, ok := ctx.Value(parentEventsKey{}).(chan<- agentTypes.Event)
	if !ok {
		parentEvents = nil
	}

	displayName, _ := sessionManager.GetBot(sessionID)
	if displayName == "" || displayName == sessionID {
		var short, rest string
		switch {
		case strings.HasPrefix(sessionID, "temp-sub-"):
			short, rest = "temp-sub-", sessionID[len("temp-sub-"):]
		case strings.HasPrefix(sessionID, "cli-"):
			short, rest = "cli-", sessionID[len("cli-"):]
		case strings.HasPrefix(sessionID, "http-"):
			short, rest = "http-", sessionID[len("http-"):]
		}
		if short != "" {
			if len(rest) > 8 {
				rest = rest[:8]
			}
			displayName = short + rest
		}
	}

	events := make(chan agentTypes.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Execute(subCtx, execData, session, events, allowAll)
		close(events)
	}()

	var sb strings.Builder
	for ev := range events {
		passSubagentEvent(parentEvents, displayName, ev)

		switch ev.Type {
		case agentTypes.EventText:
			if ev.Text == "" {
				continue
			}
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(ev.Text)
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

func passSubagentEvent(parent chan<- agentTypes.Event, name string, ev agentTypes.Event) {
	if parent == nil {
		return
	}
	switch ev.Type {
	case agentTypes.EventDone,
		agentTypes.EventAgentSelect,
		agentTypes.EventAgentResult,
		agentTypes.EventSummaryGenerate,
		agentTypes.EventToolCallStart,
		agentTypes.EventToolCallEnd,
		agentTypes.EventToolCallText,
		agentTypes.EventSkillResult:
		return
	}

	out := ev
	if out.Source == "" {
		out.Source = name
	}
	parent <- out
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
	if !go_pkg_filesystem_reader.Exists(sessionDir) {
		return "", fmt.Errorf("session %q does not exist", trimmed)
	}
	if !go_pkg_filesystem_reader.IsDir(sessionDir) {
		return "", fmt.Errorf("session %q is not a directory", trimmed)
	}
	return trimmed, nil
}

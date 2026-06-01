package exec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

func ExecWithSubagent(ctx context.Context, task, sessionIDInput, model, systemPrompt string, excludedTools []string) (string, error) {
	registry := agents.Registry()
	dispatcher := agents.DispatcherBot()
	if dispatcher == nil || len(registry.Registry) == 0 {
		return "", fmt.Errorf("subagent host not initialized")
	}

	var agent agentTypes.Agent
	if model != "" {
		agent = registry.Registry[model]
	} else {
		agent = SelectAgent(ctx, dispatcher, registry, task, false, sessionIDInput)
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
	subagentExcludeBase := []string{"invoke_subagent", "invoke_external_agent", "cross_review_with_external_agents", "review_result"}
	excluded := append(append(subagentExcludeBase, tools.TUIOnlyTools...), excludedTools...)
	execData := ExecData{
		Agent:             agent,
		WorkDir:           workDir,
		Content:           task,
		ExcludeTools:      excluded,
		ExcludeSkills:     tools.TUIOnlySkills,
		ExtraSystemPrompt: systemPrompt,
		AllowAll:          allowAll,
	}

	oldHistory, maxHistory := sessionHistory.Get(sessionID)
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
		SystemPrompts: BuildSystemPrompts(execData.WorkDir, execData.ExtraSystemPrompt, agents.Scanner(), sessionID, execData.AllowAll, false, execData.ExcludeSkills),
		OldHistories:  maxHistory,
		ToolHistories: []agentTypes.Message{},
		Tools:         []agentTypes.Message{},
		Histories:     histories,
		BaseLen:       len(oldHistory),
		UserInput:     agentTypes.Message{Role: "user", Content: userText},
	}
	if summary := summary.GetPrompt(sessionID, OldestMessageTime(maxHistory)); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	SaveUserInputHistory(sessionID, userText)

	subCtx, cancel := context.WithTimeout(ctx, time.Duration(filesystem.MaxSubagentTimeoutMin)*time.Minute)
	defer cancel()

	parentEvents, ok := ctx.Value(parentEventsKey{}).(chan<- agentTypes.Event)
	if !ok {
		parentEvents = nil
	}

	displayName, _ := configBot.Get(sessionID)
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
		str := strings.TrimSpace(sb.String())
		if str == "" {
			return "", fmt.Errorf("subagent execute: %w", err)
		}
		return fmt.Sprintf("[subagent partial result · %s · session=%s]\n%s\n\n[error] %s",
			agent.Name(), sessionID, str, err.Error()), nil
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
		id, err := sessionManager.New("temp-sub-")
		if err != nil {
			return "", fmt.Errorf("sessionManager.CreateSession: %w", err)
		}
		return id, nil
	}

	sessionDir := filesystem.SessionDir(trimmed)
	if !go_pkg_filesystem_reader.Exists(sessionDir) {
		return "", fmt.Errorf("session %q does not exist", trimmed)
	}
	if !go_pkg_filesystem_reader.IsDir(sessionDir) {
		return "", fmt.Errorf("session %q is not a directory", trimmed)
	}
	return trimmed, nil
}

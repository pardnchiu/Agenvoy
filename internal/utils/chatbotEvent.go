package utils

import (
	"fmt"
	"log/slog"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type AgentEventResult struct {
	ReplyText  string
	ExecErrors []string
	Done       agenttypes.Event
}

func FormatChatbotEvent(events <-chan agenttypes.Event, tag, sessionID string, status func(text string), err func(toolName, text string) string) AgentEventResult {
	var r AgentEventResult
	var toolCount int
	for e := range events {
		EventLog(tag, e, sessionID, "")
		switch e.Type {
		case agenttypes.EventReasoning:
			if t := strings.TrimSpace(e.Text); t != "" {
				status("[think] " + t)
			}

		case agenttypes.EventToolCall:
			if e.ToolName != "" {
				toolCount++
				status(formatChatbotToolEvent(toolCount, e))
			}

		case agenttypes.EventToolSkipped:
			if e.ToolName != "" {
				toolCount++
				status(fmt.Sprintf("[skipped #%d] %s", toolCount, e.ToolName))
			}

		case agenttypes.EventText:
			if r.ReplyText != "" {
				r.ReplyText += "\n"
			}
			r.ReplyText += e.Text

		case agenttypes.EventExecError:
			r.ExecErrors = append(r.ExecErrors, err(e.ToolName, e.Text))

		case agenttypes.EventDone:
			r.Done = e
		}
	}
	return r
}

func formatChatbotToolEvent(count int, event agenttypes.Event) string {
	return fmt.Sprintf("[tool #%d] %s", count, FormatToolEvent(event.ToolName, event.ToolArgs))
}

func EventLog(tag string, event agenttypes.Event, sessionID string, _ string) {
	if event.Type != agenttypes.EventError && event.Type != agenttypes.EventExecError {
		return
	}
	errText := event.Text
	if event.Err != nil {
		errText = event.Err.Error()
	}
	if errText == "" {
		errText = "unknown error"
	}
	sessionLog := go_pkg_utils.TruncateString(sessionID, 16)
	slog.Error(tag,
		slog.String("session", sessionLog),
		slog.String("event", event.Type.String()),
		slog.String("tool", event.ToolName),
		slog.String("error", errText))
}

package utils

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type AgentEventResult struct {
	ReplyText  string
	ExecErrors []string
	Done       agentTypes.Event
}

func FormatChatbotEvent(events <-chan agentTypes.Event, tag, sessionID string, status func(text string), err func(toolName, text string) string) AgentEventResult {
	var r AgentEventResult
	var toolCount int
	for e := range events {
		EventLog(tag, e, sessionID, "")
		switch e.Type {
		case agentTypes.EventAgentResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				status("[agent] " + go_pkg_utils.TruncateString(t, 256))
			}

		case agentTypes.EventSkillResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				status("[skill] " + go_pkg_utils.TruncateString(t, 256))
			}

		case agentTypes.EventToolCall:
			if e.ToolName != "" {
				toolCount++
				status(formatChatbotToolEvent(toolCount, e))
			}

		case agentTypes.EventToolSkipped:
			if e.ToolName != "" {
				toolCount++
				status(fmt.Sprintf("[skipped #%d] %s", toolCount, e.ToolName))
			}

		case agentTypes.EventText:
			if r.ReplyText != "" {
				r.ReplyText += "\n"
			}
			r.ReplyText += e.Text

		case agentTypes.EventExecError:
			r.ExecErrors = append(r.ExecErrors, err(e.ToolName, e.Text))

		case agentTypes.EventDone:
			r.Done = e
		}
	}
	return r
}

func formatChatbotToolEvent(count int, event agentTypes.Event) string {
	body := event.ToolName + "(" + go_pkg_utils.TruncateString(event.ToolArgs, 256) + ")"
	switch event.ToolName {
	case "fetch_page":
		var p struct {
			Link string `json:"link"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(event.ToolArgs), &p); err != nil {
			return "Fetch(" + go_pkg_utils.TruncateString(event.ToolArgs, 256) + ")"
		}
		body = "Fetch(" + p.Link + " " + p.Type + ")"
	}
	return fmt.Sprintf("[tool #%d] %s", count, body)
}

func EventLog(tag string, event agentTypes.Event, sessionID string, _ string) {
	if event.Type != agentTypes.EventError && event.Type != agentTypes.EventExecError {
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

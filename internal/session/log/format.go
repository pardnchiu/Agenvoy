package sessionLog

import (
	"fmt"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	maxActionLogSize = 1 << 20
	trimTargetSize   = 768 << 10
)

func formatActionEvent(event agentTypes.Event) string {
	switch event.Type {
	case agentTypes.EventText:
		text := strings.TrimSpace(event.Text)
		if text == "" {
			return ""
		}
		return withTimestamp("assistant", flatten(text))

	case agentTypes.EventToolCall:
		body := event.ToolName
		if display := utils.FormatToolEvent(event.ToolName, event.ToolArgs); display != "" {
			body = fmt.Sprintf("%s %s", body, flatten(display))
		}
		return withTimestamp("tool_call", body)

	case agentTypes.EventToolResult:
		status := "ok"
		if event.Err != nil {
			status = "err"
		}
		return withTimestamp("tool_result", fmt.Sprintf("%s %s", event.ToolName, status))

	case agentTypes.EventToolSkipped:
		return withTimestamp("tool_skipped", event.ToolName)

	case agentTypes.EventToolConfirm:
		return withTimestamp("tool_confirm", event.ToolName)

	case agentTypes.EventExecError, agentTypes.EventError:
		body := ""
		if event.Err != nil {
			body = flatten(event.Err.Error())
		} else if event.Text != "" {
			body = flatten(event.Text)
		} else {
			return ""
		}
		if event.ToolName != "" {
			body = fmt.Sprintf("%s %s", event.ToolName, body)
		}
		return withTimestamp("error", body)

	case agentTypes.EventDone:
		parts := []string{event.Model}
		if event.Duration > 0 {
			parts = append(parts, fmt.Sprintf("dur=%s", event.Duration.Round(time.Millisecond)))
		}
		if event.Usage != nil {
			parts = append(parts, fmt.Sprintf("in=%d", event.Usage.Input), fmt.Sprintf("out=%d", event.Usage.Output))
		}
		return withTimestamp("done", strings.Join(parts, " "))

	case agentTypes.EventSkillResult:
		text := strings.TrimSpace(event.Text)
		if text == "" {
			return ""
		}
		return withTimestamp("skill_result", flatten(text))

	case agentTypes.EventAgentResult:
		text := strings.TrimSpace(event.Text)
		if text == "" {
			return ""
		}
		return withTimestamp("agent_result", flatten(text))
	}
	return ""
}

func withTimestamp(kind, body string) string {
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	return fmt.Sprintf("[%s][%s][%s] %s", ts, session.GetHash(), kind, body)
}

func flatten(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", ActionNewlineMarker)
	return text
}

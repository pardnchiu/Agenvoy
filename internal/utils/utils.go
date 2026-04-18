package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func NewID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
}

func EventLog(tag string, event agentTypes.Event, sessionID string, input string) {
	sessionLog := sessionID
	if len(sessionLog) > 16 {
		sessionLog = sessionLog[:13] + "…"
	}

	if input != "" {
		inputLog := input
		if len(inputLog) > 32 {
			inputLog = inputLog[:31] + "…"
		}
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("input", inputLog))
		return
	}

	switch event.Type {
	case agentTypes.EventAgentSelect:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()))

	case agentTypes.EventSkillResult:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("skill", event.Text))

	case agentTypes.EventAgentResult:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("agent", event.Text))

	case agentTypes.EventToolCall:
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("tool", event.ToolName))

	case agentTypes.EventText:
		text := event.Text
		if len(text) > 32 {
			text = text[:31] + "…"
		}
		slog.Info(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("output", text))

	case agentTypes.EventError, agentTypes.EventExecError:
		errText := event.Text
		if event.Err != nil {
			errText = event.Err.Error()
		}
		if errText == "" {
			errText = "unknown error"
		}
		slog.Error(tag,
			slog.String("session", sessionLog),
			slog.String("event", event.Type.String()),
			slog.String("tool", event.ToolName),
			slog.String("error", errText))

	default:
		break
	}
}

func FormatInt(number int) string {
	s := fmt.Sprintf("%d", number)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

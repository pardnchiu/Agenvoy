package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func Keys[V any](obj map[string]V) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

func NewID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
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
	sessionLog := sessionID
	if len(sessionLog) > 16 {
		sessionLog = sessionLog[:13] + "…"
	}
	slog.Error(tag,
		slog.String("session", sessionLog),
		slog.String("event", event.Type.String()),
		slog.String("tool", event.ToolName),
		slog.String("error", errText))
}

var (
	uuidShortRegex   = regexp.MustCompile(`([0-9a-fA-F]{8})-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	sha256ShortRegex = regexp.MustCompile(`\b([0-9a-fA-F]{8})[0-9a-fA-F]{56}\b`)
)

func ShortenSessionID(sid string) string {
	sid = uuidShortRegex.ReplaceAllString(sid, "$1")
	sid = sha256ShortRegex.ReplaceAllString(sid, "$1")
	return sid
}

func TruncateStatus(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return string(r)
}

func CompactNumber(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func FormatFooter(duration time.Duration, model string, usage *agentTypes.Usage) string {
	var parts []string
	if duration > 0 {
		parts = append(parts, duration.Round(100*time.Millisecond).String())
	}
	if model = strings.TrimSpace(model); model != "" {
		if _, after, ok := strings.Cut(model, "@"); ok {
			model = after
		}
		parts = append(parts, model)
	}
	if usage != nil && (usage.Input > 0 || usage.Output > 0) {
		parts = append(parts, fmt.Sprintf("↑%s ↓%s", CompactNumber(usage.Input), CompactNumber(usage.Output)))
	}
	return strings.Join(parts, " · ")
}

type AgentEventResult struct {
	ReplyText  string
	ExecErrors []string
	Done       agentTypes.Event
}

func FormatAgentEventMessage(
	events <-chan agentTypes.Event,
	tag, sessionID string,
	markStatus func(text string),
	execErrFmt func(toolName, text string) string,
) AgentEventResult {
	var r AgentEventResult
	var toolCount int
	for e := range events {
		EventLog(tag, e, sessionID, "")
		switch e.Type {
		case agentTypes.EventAgentResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				markStatus("[agent] " + TruncateStatus(t))
			}

		case agentTypes.EventSkillResult:
			if t := strings.TrimSpace(e.Text); t != "" {
				markStatus("[skill] " + TruncateStatus(t))
			}

		case agentTypes.EventToolCall:
			if e.ToolName != "" {
				toolCount++
				markStatus(formatToolEvent(toolCount, e))
			}

		case agentTypes.EventToolSkipped:
			if e.ToolName != "" {
				toolCount++
				markStatus(fmt.Sprintf("[skipped #%d] %s", toolCount, e.ToolName))
			}

		case agentTypes.EventText:
			if r.ReplyText != "" {
				r.ReplyText += "\n"
			}
			r.ReplyText += e.Text

		case agentTypes.EventExecError:
			r.ExecErrors = append(r.ExecErrors, execErrFmt(e.ToolName, e.Text))

		case agentTypes.EventDone:
			r.Done = e
		}
	}
	return r
}

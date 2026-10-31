package sessionLog

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	lineRegex     = regexp.MustCompile(`^\[([^\]]+)\]\[([^\]]*)\]\[([^\]]+)\](?:\[([^\]]*)\])?\s*(.*)$`)
	metaWrapRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n`)
)

func RecentEvents(sessionID string, limit int) []agenttypes.Event {
	text, err := go_pkg_filesystem.ReadText(filesystem.ActionLogPath(sessionID))
	if err != nil || text == "" {
		return nil
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	events := make([]agenttypes.Event, 0, len(lines))
	for _, line := range lines {
		if ev, ok := ParseLine(line); ok {
			events = append(events, ev)
		}
	}
	return events
}

func LastTaskHash(sessionID string) string {
	text, err := go_pkg_filesystem.ReadText(filesystem.ActionLogPath(sessionID))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if m := lineRegex.FindStringSubmatch(lines[i]); len(m) >= 6 && m[4] != "" {
			return m[4]
		}
	}
	return ""
}

func ParseLine(line string) (agenttypes.Event, bool) {
	m := lineRegex.FindStringSubmatch(line)
	if len(m) < 6 {
		return agenttypes.Event{}, false
	}
	kind := m[3]
	body := strings.ReplaceAll(m[5], ActionNewlineMarker, "\n")

	switch kind {
	case "user", "steer":
		body = metaWrapRegex.ReplaceAllString(body, "")
		if strings.Contains(body, "[Resumed Task") || strings.HasPrefix(body, "[Scheduled Task") {
			return agenttypes.Event{}, false
		}
		body = strings.TrimSpace(body)
		if body == "" {
			return agenttypes.Event{}, false
		}
		return agenttypes.Event{Type: agenttypes.EventUserInput, Text: body}, true
	case "assistant":
		return agenttypes.Event{Type: agenttypes.EventTextDone, Text: body}, true
	case "thinking":
		return agenttypes.Event{Type: agenttypes.EventReasoning, Text: body}, true
	case "todo":
		var todos []agenttypes.TodoItem
		if err := json.Unmarshal([]byte(body), &todos); err != nil || len(todos) == 0 {
			return agenttypes.Event{}, false
		}
		return agenttypes.Event{Type: agenttypes.EventTodoUpdate, Todos: todos}, true
	case "tool_call":
		name, args, _ := strings.Cut(body, " ")
		return agenttypes.Event{Type: agenttypes.EventToolCall, ToolName: name, ToolArgs: args}, true
	case "tool_result":
		name, status, _ := strings.Cut(body, " ")
		ev := agenttypes.Event{Type: agenttypes.EventToolResult, ToolName: name, Result: status}
		if strings.HasPrefix(status, "err") {
			ev.Err = errors.New("tool error")
		}
		return ev, true
	case "tool_skipped":
		name, args, _ := strings.Cut(body, " ")
		return agenttypes.Event{Type: agenttypes.EventToolSkipped, ToolName: name, ToolArgs: args}, true
	case "tool_confirm":
		return agenttypes.Event{Type: agenttypes.EventToolConfirm, ToolName: body}, true
	case "error":
		name, msg, _ := strings.Cut(body, " ")
		return agenttypes.Event{Type: agenttypes.EventExecError, ToolName: name, Text: msg, Err: errors.New(msg)}, true
	case "done":
		return parseDone(body), true
	case "canceled":
		event := parseDone(body)
		event.Type = agenttypes.EventCanceled
		return event, true
	case "skill_result":
		return agenttypes.Event{Type: agenttypes.EventSkillResult, Text: body}, true
	case "agent_result":
		return agenttypes.Event{Type: agenttypes.EventAgentResult, Text: body}, true
	}
	return agenttypes.Event{}, false
}

func parseDone(body string) agenttypes.Event {
	event := agenttypes.Event{Type: agenttypes.EventDone}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return event
	}
	if !strings.Contains(fields[0], "=") {
		event.Model = fields[0]
		fields = fields[1:]
	}

	var usage go_llm_router.Usage
	var hasUsage bool
	hitPct := -1
	for _, f := range fields {
		if m := configs.CACHE_HIT_PCT_REGEX.FindStringSubmatch(f); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				hitPct = n
			}
			continue
		}
		k, v, found := strings.Cut(f, "=")
		if !found {
			continue
		}
		switch k {
		case "reasoning":
			event.Reasoning = v
		case "dur":
			if d, err := time.ParseDuration(v); err == nil {
				event.Duration = d
			}
		case "in":
			if n, err := strconv.Atoi(v); err == nil {
				usage.Input = n
				hasUsage = true
			}
		case "out":
			if n, err := strconv.Atoi(v); err == nil {
				usage.Output = n
				hasUsage = true
			}
		}
	}
	if hitPct >= 0 && usage.Input > 0 {
		usage.CacheRead = usage.Input * hitPct / 100
		usage.Input -= usage.CacheRead
	}
	if hasUsage {
		event.Usage = &usage
	}
	return event
}

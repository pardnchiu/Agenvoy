package sessionLog

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

var (
	lineRegex     = regexp.MustCompile(`^\[([^\]]+)\]\[([^\]]*)\]\[([^\]]+)\]\s*(.*)$`)
	metaWrapRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n`)
)

func RecentEvents(sessionID string, limit int) []agentTypes.Event {
	text, err := go_pkg_filesystem.ReadText(filesystem.ActionLogPath(sessionID))
	if err != nil || text == "" {
		return nil
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	events := make([]agentTypes.Event, 0, len(lines))
	for _, line := range lines {
		if ev, ok := ParseLine(line); ok {
			events = append(events, ev)
		}
	}
	return events
}

func ParseLine(line string) (agentTypes.Event, bool) {
	m := lineRegex.FindStringSubmatch(line)
	if len(m) < 5 {
		return agentTypes.Event{}, false
	}
	kind := m[3]
	body := strings.ReplaceAll(m[4], ActionNewlineMarker, "\n")

	switch kind {
	case "user":
		body = metaWrapRegex.ReplaceAllString(body, "")
		if strings.Contains(body, "[Resumed Task") {
			return agentTypes.Event{}, false
		}
		body = strings.TrimSpace(body)
		if body == "" {
			return agentTypes.Event{}, false
		}
		return agentTypes.Event{Type: agentTypes.EventUserInput, Text: body}, true
	case "assistant":
		return agentTypes.Event{Type: agentTypes.EventTextDone, Text: body}, true
	case "tool_call":
		name, args, _ := strings.Cut(body, " ")
		return agentTypes.Event{Type: agentTypes.EventToolCall, ToolName: name, ToolArgs: args}, true
	case "tool_result":
		name, status, _ := strings.Cut(body, " ")
		ev := agentTypes.Event{Type: agentTypes.EventToolResult, ToolName: name, Result: status}
		if strings.HasPrefix(status, "err") {
			ev.Err = errors.New("tool error")
		}
		return ev, true
	case "tool_skipped":
		name, args, _ := strings.Cut(body, " ")
		return agentTypes.Event{Type: agentTypes.EventToolSkipped, ToolName: name, ToolArgs: args}, true
	case "tool_confirm":
		return agentTypes.Event{Type: agentTypes.EventToolConfirm, ToolName: body}, true
	case "error":
		name, msg, _ := strings.Cut(body, " ")
		return agentTypes.Event{Type: agentTypes.EventExecError, ToolName: name, Text: msg, Err: errors.New(msg)}, true
	case "done":
		return parseDone(body), true
	case "skill_result":
		return agentTypes.Event{Type: agentTypes.EventSkillResult, Text: body}, true
	case "agent_result":
		return agentTypes.Event{Type: agentTypes.EventAgentResult, Text: body}, true
	}
	return agentTypes.Event{}, false
}

func parseDone(body string) agentTypes.Event {
	event := agentTypes.Event{Type: agentTypes.EventDone}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return event
	}
	if !strings.Contains(fields[0], "=") {
		event.Model = fields[0]
		fields = fields[1:]
	}

	var usage agentTypes.Usage
	var hasUsage bool
	for _, f := range fields {
		k, v, found := strings.Cut(f, "=")
		if !found {
			continue
		}
		switch k {
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
	if hasUsage {
		event.Usage = &usage
	}
	return event
}

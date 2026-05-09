package tui

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session"
)

var userWrapperRe = regexp.MustCompile(`^---\n當前時間:[^\n]*\n工作目錄:[^\n]*\n---\n`)

func formatLog(raw string) string {
	if raw == "" {
		return ""
	}

	kind, body, ok := splitLine(raw)
	if !ok {
		return ""
	}

	body = strings.ReplaceAll(body, session.ActionNewlineMarker, "\n")

	switch kind {
	case "user":
		body = userWrapperRe.ReplaceAllString(body, "")
		body = strings.TrimSpace(body)
		if body == "" {
			return ""
		}
		return messageBlock(body)

	case "assistant":
		text := strings.TrimSpace(body)
		if text == "" {
			return ""
		}
		return renderEvent(agentTypes.Event{Type: agentTypes.EventText, Text: text})

	case "tool_call":
		name, args, _ := strings.Cut(body, " ")
		return renderEvent(agentTypes.Event{
			Type:     agentTypes.EventToolCall,
			ToolName: name,
			ToolArgs: args,
		})

	case "tool_skipped":
		name, args, _ := strings.Cut(body, " ")
		return renderEvent(agentTypes.Event{
			Type:     agentTypes.EventToolSkipped,
			ToolName: name,
			ToolArgs: args,
		})

	case "error":
		name, msg, _ := strings.Cut(body, " ")
		return renderEvent(agentTypes.Event{
			Type:     agentTypes.EventExecError,
			ToolName: name,
			Text:     msg,
			Err:      errors.New(msg),
		})

	case "done":
		return renderEvent(formatDone(body))

	case "skill_result":
		text := strings.TrimSpace(body)
		if text == "" {
			return ""
		}
		return renderEvent(agentTypes.Event{Type: agentTypes.EventSkillResult, Text: text})
	}
	return ""
}

func splitLine(raw string) (kind, body string, ok bool) {
	if !strings.HasPrefix(raw, "[") {
		return "", "", false
	}

	kindEndIdx := 0
	if _, after, found := strings.Cut(raw, "]"); found {
		kindEndIdx = len(raw) - len(after) - 1
	} else {
		return "", "", false
	}

	i := kindEndIdx
	if i < 0 {
		return "", "", false
	}

	rest := raw[i+1:]
	if !strings.HasPrefix(rest, "[") {
		return "", "", false
	}

	j := strings.Index(rest, "]")
	if j < 0 {
		return "", "", false
	}
	kind = rest[1:j]
	body = strings.TrimSpace(rest[j+1:])
	return kind, body, true
}

func renderEvent(ev agentTypes.Event) string {
	line, ok := renderAgentEvent(ev, "", "")
	if !ok {
		return ""
	}
	return line
}

func formatDone(body string) agentTypes.Event {
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

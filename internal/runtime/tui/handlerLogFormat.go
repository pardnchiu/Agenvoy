package tui

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
	sessionTUI "github.com/pardnchiu/agenvoy/internal/session/tui"
)

var userWrapperRe = regexp.MustCompile(`^---\n當前時間:[^\n]*\n(?:[^\n]+\n)*?---\n`)

type parsedAction struct {
	hash string
	kind string
	body string
}

func cutBracket(s string) (inside, rest string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", false
	}
	inside, rest, ok = strings.Cut(s[1:], "]")
	return
}

func parseActionLine(raw string) (parsedAction, bool) {
	_, rest, ok := cutBracket(raw)
	if !ok {
		return parsedAction{}, false
	}
	mid, rest, ok := cutBracket(rest)
	if !ok {
		return parsedAction{}, false
	}

	var hash, kind string
	if third, after, ok := cutBracket(rest); ok {
		hash = mid
		kind = third
		rest = after
	} else {
		hash = sessionTUI.Default
		kind = mid
	}

	return parsedAction{
		hash: hash,
		kind: kind,
		body: strings.TrimSpace(rest),
	}, true
}

func renderActionLine(p parsedAction) string {
	body := strings.ReplaceAll(p.body, sessionLog.ActionNewlineMarker, "\n")

	switch p.kind {
	case "user":
		body = userWrapperRe.ReplaceAllString(body, "")
		body = strings.TrimSpace(body)
		if body == "" {
			return ""
		}
		return messageBlock(body)

	case "assistant":
		str := strings.TrimSpace(body)
		if str == "" {
			return ""
		}
		return renderEvent(agentTypes.Event{Type: agentTypes.EventText, Text: str})

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
		str := strings.TrimSpace(body)
		if str == "" {
			return ""
		}
		return renderEvent(agentTypes.Event{Type: agentTypes.EventSkillResult, Text: str})
	}
	return ""
}

func formatLog(raw string) string {
	p, ok := parseActionLine(raw)
	if !ok {
		return ""
	}
	return renderActionLine(p)
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

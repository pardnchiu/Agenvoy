package tui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
)

type parsedAction struct {
	timestamp string
	hash      string
	kind      string
	body      string
}

func cutBracket(s string) (inside, rest string, ok bool) {
	if !strings.HasPrefix(s, "[") {
		return "", "", false
	}
	inside, rest, ok = strings.Cut(s[1:], "]")
	return
}

func parseActionLine(raw string) (parsedAction, bool) {
	ts, rest, ok := cutBracket(raw)
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
		if _, tail, ok := cutBracket(rest); ok {
			rest = tail
		}
	} else {
		hash = sessiontui.Default
		kind = mid
	}

	return parsedAction{
		timestamp: ts,
		hash:      hash,
		kind:      kind,
		body:      strings.TrimSpace(rest),
	}, true
}

func renderActionLine(p parsedAction, width int) string {
	body := strings.ReplaceAll(p.body, sessionlog.ActionNewlineMarker, "\n")

	switch p.kind {
	case "user", "steer":
		body = configs.MESSAGE_PREFIX_REGEX.ReplaceAllString(body, "")
		if strings.Contains(body, "[Resumed Task") || strings.HasPrefix(body, "[Shell command]") {
			return ""
		}
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
		return renderEvent(agenttypes.Event{Type: agenttypes.EventText, Text: str}, width)

	case "tool_skipped":
		name, args, _ := strings.Cut(body, " ")
		return renderEvent(agenttypes.Event{
			Type:     agenttypes.EventToolSkipped,
			ToolName: name,
			ToolArgs: args,
		}, width)

	case "error":
		name, msg, _ := strings.Cut(body, " ")
		return renderEvent(agenttypes.Event{
			Type:     agenttypes.EventExecError,
			ToolName: name,
			Text:     msg,
			Err:      errors.New(msg),
		}, width)

	case "done":
		return renderEvent(formatDone(body), width, formatLogTimestamp(p.timestamp))

	case "canceled":
		event := formatDone(body)
		event.Type = agenttypes.EventCanceled
		return renderEvent(event, width, formatLogTimestamp(p.timestamp))

	case "skill_result":
		str := strings.TrimSpace(body)
		if str == "" {
			return ""
		}
		return renderEvent(agenttypes.Event{Type: agenttypes.EventSkillResult, Text: str}, width)
	}
	return ""
}

func formatLogTimestamp(ts string) string {
	if t, err := time.ParseInLocation("2006-01-02 15:04:05.000", ts, time.Local); err == nil {
		return formatFinishedAt(t)
	}
	return ts
}

func formatFinishedAt(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func formatLog(raw string, width int) (kind, line string) {
	p, ok := parseActionLine(raw)
	if !ok {
		return "", ""
	}
	return p.kind, renderActionLine(p, width)
}

func renderEvent(ev agenttypes.Event, width int, finishedAt ...string) string {
	ts := ""
	if len(finishedAt) > 0 {
		ts = finishedAt[0]
	}
	line, ok := renderAgentEvent(ev, "", "", width, ts)
	if !ok {
		return ""
	}
	return line
}

func formatDone(body string) agenttypes.Event {
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
		case "outdur":
			if d, err := time.ParseDuration(v); err == nil {
				event.OutputElapsed = d
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

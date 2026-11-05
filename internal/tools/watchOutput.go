package tools

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type watchSpec struct {
	interval time.Duration
	prompt   string
	agent    go_llm_router.DecisionAgent
	model    string
}

func parseWatch(interval int, prompt string) (watchSpec, error) {
	spec := watchSpec{prompt: strings.TrimSpace(prompt)}
	if spec.prompt == "" {
		return spec, utilsmsg.New(utilsmsg.Invalid, "prompt", "required")
	}
	if interval < configs.WATCH_MIN_INTERVAL_SEC {
		return spec, utilsmsg.New(utilsmsg.Invalid, "interval", "must be at least %d seconds", configs.WATCH_MIN_INTERVAL_SEC)
	}
	spec.interval = time.Duration(interval) * time.Second

	agent, model, err := decision.Agent(context.Background())
	if err != nil {
		return spec, utilsmsg.Trace("decision.Agent", "interval checks need a decision model to judge prompt", err)
	}
	spec.agent, spec.model = agent, model
	return spec, nil
}

func watchOutput(ctx context.Context, spec watchSpec, buf *watchBuffer, done <-chan struct{}, report func(string)) (int, string, error) {
	ticker := time.NewTicker(spec.interval)
	defer ticker.Stop()

	var last, lastReport string
	var lastReportAt time.Time
	failures, reports := 0, 0
	for {
		select {
		case <-ctx.Done():
			return reports, last, nil

		case <-done:
			return reports, last, nil

		case <-ticker.C:
			if !lastReportAt.IsZero() && time.Since(lastReportAt) < time.Duration(configs.WATCH_REPORT_COOLDOWN_SEC)*time.Second {
				continue
			}
			lines, dropped := buf.take()
			if len(lines) == 0 {
				continue
			}
			text := formatWatchLines(lines, dropped)
			last = text

			hit, confidence, sent, err := decideWatch(ctx, spec, lines, lastReport, lastReportAt)
			if !sent {
				buf.restore(lines, dropped)
			}
			if err != nil {
				if ctx.Err() != nil {
					continue
				}
				failures++
				if failures >= configs.WATCH_MAX_DECIDE_FAILURES {
					pending, lost := buf.take()
					return reports, formatWatchLines(pending, lost), err
				}
				continue
			}
			failures = 0
			if hit {
				reports++
				lastReport, lastReportAt = text, time.Now()
				report(fmt.Sprintf("condition met at %s (confidence %.2f, report #%d); still running\n%s", time.Now().Format("15:04:05"), confidence, reports, text))
			}
		}
	}
}

func decideWatch(ctx context.Context, spec watchSpec, lines []string, lastReport string, lastReportAt time.Time) (bool, float64, bool, error) {
	state := map[string]any{
		"condition": spec.prompt,
		"logs":      lines,
	}
	questions := map[string]go_llm_router.DecisionQuestion{
		"trigger": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Do `logs` satisfy `condition`?",
				"focus":    "Judge only the lines in `logs`; each line is one stream message or one line of command output, prefixed with its arrival time.",
			},
			Criteria: map[string]any{
				"yes": "At least one line in `logs` meets `condition`.",
				"no":  "Nothing in `logs` meets `condition`.",
			},
		},
	}
	if !lastReportAt.IsZero() {
		state["last_report"] = map[string]any{
			"time": lastReportAt.Format("15:04:05"),
			"logs": lastReport,
		}
		questions["repeat"] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Do `logs` only repeat the situation already reported in `last_report`?",
				"focus":    "`last_report` holds the lines already reported for `condition`. The condition simply staying met with similar values is a repeat.",
			},
			Criteria: map[string]any{
				"repeat": "`condition` stays met the same way as in `last_report`; nothing new to report.",
				"new":    "`logs` show `condition` met again after it stopped holding, or met in a clearly different way than `last_report`.",
			},
		}
	}

	answers, err := decision.Decide(ctx, spec.agent, state, questions)
	if err != nil {
		return false, 0, false, utilsmsg.Trace("Decide", spec.model, err)
	}
	answer, ok := answers["trigger"]
	if !ok {
		return false, 0, true, utilsmsg.New(utilsmsg.Upstream, spec.model, "response has no trigger answer")
	}
	confidence := 0.0
	if answer.Confidence != nil {
		confidence = *answer.Confidence
	}
	switch answer.Choice {
	case "yes":
	case "no":
		return false, confidence, true, nil
	default:
		return false, 0, true, utilsmsg.New(utilsmsg.Upstream, spec.model, "invalid choice %q", answer.Choice)
	}
	if confidence < configs.WATCH_MIN_CONFIDENCE {
		return false, confidence, true, nil
	}
	if lastReportAt.IsZero() {
		return true, confidence, true, nil
	}

	repeat, ok := answers["repeat"]
	if !ok {
		return false, 0, true, utilsmsg.New(utilsmsg.Upstream, spec.model, "response has no repeat answer")
	}
	switch repeat.Choice {
	case "new":
		return repeat.Confidence != nil && *repeat.Confidence >= configs.WATCH_MIN_CONFIDENCE, confidence, true, nil
	case "repeat":
		return false, confidence, true, nil
	}
	return false, 0, true, utilsmsg.New(utilsmsg.Upstream, spec.model, "invalid repeat choice %q", repeat.Choice)
}

func formatWatchLines(lines []string, dropped int) string {
	if len(lines) == 0 {
		return ""
	}
	text := strings.Join(lines, "\n")
	if dropped > 0 {
		text = fmt.Sprintf("...[%d earlier lines dropped]...\n", dropped) + text
	}
	return text
}

type watchBuffer struct {
	mu      sync.Mutex
	lines   []string
	size    int
	dropped int
	total   int
	partial []byte
	limit   int
}

func (b *watchBuffer) Write(raw []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.partial = append(b.partial, raw...)
	for {
		i := bytes.IndexByte(b.partial, '\n')
		if i < 0 {
			break
		}
		b.addLocked(string(b.partial[:i]))
		b.partial = b.partial[i+1:]
	}
	if len(b.partial) > configs.WATCH_LINE_MAX_BYTES {
		b.addLocked(string(b.partial))
		b.partial = nil
	}
	return len(raw), nil
}

func (b *watchBuffer) add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addLocked(line)
}

func (b *watchBuffer) addLocked(line string) {
	b.total++
	if len(line) > configs.WATCH_LINE_MAX_BYTES {
		line = strings.ToValidUTF8(line[:configs.WATCH_LINE_MAX_BYTES], "") + "...[truncated]"
	}
	line = time.Now().Format("15:04:05") + " " + line
	b.lines = append(b.lines, line)
	b.size += len(line)
	b.trimLocked()
}

func (b *watchBuffer) trimLocked() {
	for b.size > b.limit && len(b.lines) > 1 {
		b.size -= len(b.lines[0])
		b.lines = b.lines[1:]
		b.dropped++
	}
}

func (b *watchBuffer) restore(lines []string, dropped int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(slices.Clone(lines), b.lines...)
	for _, line := range lines {
		b.size += len(line)
	}
	b.dropped += dropped
	b.trimLocked()
}

func (b *watchBuffer) count() {
	b.mu.Lock()
	b.total++
	b.mu.Unlock()
}

func (b *watchBuffer) take() ([]string, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lines, dropped := b.lines, b.dropped
	b.lines, b.size, b.dropped = nil, 0, 0
	return lines, dropped
}

func (b *watchBuffer) received() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.total
}

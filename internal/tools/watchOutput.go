package tools

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type watchSpec struct {
	keywords []string
	agent    go_llm_router.DecisionAgent
	model    string
}

func newWatch(keywords []string) watchSpec {
	spec := watchSpec{}
	for _, keyword := range keywords {
		if keyword = strings.ToLower(strings.TrimSpace(keyword)); keyword != "" {
			spec.keywords = append(spec.keywords, keyword)
		}
	}
	if len(spec.keywords) == 0 {
		spec.keywords = configs.WATCH_KEYWORDS
	}
	if agent, model, ok := decision.Ready(context.Background(), "tool", "background_command"); ok {
		spec.agent, spec.model = agent, model
	}
	return spec
}

func watchOutput(ctx context.Context, spec watchSpec, buf *watchBuffer, done <-chan struct{}, report func(string)) {
	settle := time.Duration(configs.WATCH_SETTLE_SEC) * time.Second
	settleMax := time.Duration(configs.WATCH_SETTLE_MAX_SEC) * time.Second
	cooldown := time.Duration(configs.WATCH_REPORT_COOLDOWN_SEC) * time.Second
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	wait := func(until func() bool) bool {
		for !until() {
			select {
			case <-ctx.Done():
				return false
			case <-done:
				return false
			case <-ticker.C:
			}
		}
		return true
	}

	var lastReport string
	var lastReportAt time.Time
	reports := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-buf.hit:
		}

		hitAt := time.Now()
		settled := wait(func() bool {
			return time.Since(buf.lastWrite()) >= settle || time.Since(hitAt) >= settleMax
		})
		if !settled || !wait(func() bool { return time.Since(lastReportAt) >= cooldown }) {
			return
		}

		lines, dropped := buf.take()
		segment := tailWatchLines(lines, dropped, configs.WATCH_REPORT_MAX_BYTES-watchHeaderBytes)
		if segment == "" {
			continue
		}

		note := ""
		if spec.agent != nil {
			send, err := decideWatch(ctx, spec, segment, lastReport, lastReportAt)
			if ctx.Err() != nil {
				return
			}
			switch {
			case err != nil:
				note = fmt.Sprintf(" (decision check failed, reported unchecked: %v)", err)
			case !send:
				continue
			}
		}

		reports++
		lastReport, lastReportAt = segment, time.Now()
		report(fmt.Sprintf("error keyword matched at %s (report #%d); the command keeps running%s. Explain what went wrong from the last output below:\n%s", hitAt.Format("15:04:05"), reports, note, segment))
	}
}

const watchHeaderBytes = 512

func decideWatch(ctx context.Context, spec watchSpec, segment, lastReport string, lastReportAt time.Time) (bool, error) {
	state := map[string]any{
		"keywords": spec.keywords,
		"logs":     segment,
	}
	questions := map[string]go_llm_router.DecisionQuestion{
		"problem": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Do `logs` show an actual error or failure?",
				"focus":    "`logs` is the latest output of a running command, each line prefixed with its arrival time; it was flagged because a line contains one of `keywords`.",
			},
			Criteria: map[string]any{
				"yes": "A line reports a real error, failure, crash or exception.",
				"no":  "The keyword only appears in a benign context, such as `0 errors`, a file or flag name, or a passing summary.",
			},
		},
	}
	if !lastReportAt.IsZero() {
		if runes := []rune(lastReport); len(runes) > configs.MAX_DECISION_RUNES {
			lastReport = string(runes[len(runes)-configs.MAX_DECISION_RUNES:])
		}
		state["last_report"] = map[string]any{
			"time": lastReportAt.Format("15:04:05"),
			"logs": lastReport,
		}
		questions["repeat"] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Are the problems in `logs` all the same kind as those already reported in `last_report`?",
				"focus":    "`logs` may hold several errors accumulated since the last report. Compare each error itself (type, message, failing file or component), ignoring timestamps and line numbers that shift between rebuilds.",
			},
			Criteria: map[string]any{
				"same": "Every error in `logs` is one already reported in `last_report`, recurring or still unresolved.",
				"new":  "At least one error in `logs` is of a different kind, or the same component fails for a different reason.",
			},
		}
	}

	answers, err := decision.Decide(ctx, spec.agent, state, questions)
	if err != nil {
		return false, utilsmsg.Trace("Decide", spec.model, err)
	}
	answer, ok := answers["problem"]
	if !ok {
		return false, utilsmsg.New(utilsmsg.Upstream, spec.model, "response has no problem answer")
	}
	if !decision.Confident(answer, "yes") {
		return false, nil
	}
	if lastReportAt.IsZero() {
		return true, nil
	}
	repeat, ok := answers["repeat"]
	if !ok {
		return false, utilsmsg.New(utilsmsg.Upstream, spec.model, "response has no repeat answer")
	}
	return decision.Confident(repeat, "new"), nil
}

func tailWatchLines(lines []string, dropped, limit int) string {
	if len(lines) == 0 {
		return ""
	}
	size, start := 0, len(lines)
	for start > 0 && (start == len(lines) || size+len(lines[start-1])+1 <= limit) {
		size += len(lines[start-1]) + 1
		start--
	}
	dropped += start
	text := strings.Join(lines[start:], "\n")
	if dropped > 0 {
		text = fmt.Sprintf("...[%d earlier lines dropped]...\n", dropped) + text
	}
	return text
}

type watchBuffer struct {
	mu       sync.Mutex
	lines    []string
	size     int
	dropped  int
	partial  []byte
	limit    int
	keywords []string
	written  time.Time
	hit      chan struct{}
}

func newWatchBuffer(keywords []string) *watchBuffer {
	return &watchBuffer{limit: configs.WATCH_BUFFER_MAX_BYTES, keywords: keywords, hit: make(chan struct{}, 1)}
}

func (b *watchBuffer) Write(raw []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.written = time.Now()
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

func (b *watchBuffer) addLocked(line string) {
	if len(line) > configs.WATCH_LINE_MAX_BYTES {
		line = strings.ToValidUTF8(line[:configs.WATCH_LINE_MAX_BYTES], "") + "...[truncated]"
	}
	lower := strings.ToLower(line)
	for _, keyword := range b.keywords {
		if strings.Contains(lower, keyword) {
			select {
			case b.hit <- struct{}{}:
			default:
			}
			break
		}
	}
	line = time.Now().Format("15:04:05") + " " + line
	b.lines = append(b.lines, line)
	b.size += len(line)
	for b.size > b.limit && len(b.lines) > 1 {
		b.size -= len(b.lines[0])
		b.lines = b.lines[1:]
		b.dropped++
	}
}

func (b *watchBuffer) lastWrite() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.written
}

func (b *watchBuffer) take() ([]string, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	lines, dropped := b.lines, b.dropped
	b.lines, b.size, b.dropped = nil, 0, 0
	return lines, dropped
}

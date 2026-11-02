package tools

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type watchSpec struct {
	interval time.Duration
	prompt   string
	apiKey   string
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

	spec.apiKey = strings.TrimSpace(keychain.Get(config.TypesafeKey))
	if spec.apiKey == "" {
		return spec, utilsmsg.New(utilsmsg.Denied, config.TypesafeKey, "missing in keychain; interval checks need it to judge prompt")
	}
	return spec, nil
}

func watchOutput(ctx context.Context, spec watchSpec, buf *watchBuffer, done <-chan struct{}, report func(string)) (int, string, error) {
	ticker := time.NewTicker(spec.interval)
	defer ticker.Stop()

	var last string
	failures, reports := 0, 0
	for {
		select {
		case <-ctx.Done():
			return reports, last, nil

		case <-done:
			return reports, last, nil

		case <-ticker.C:
			lines, dropped := buf.take()
			if len(lines) == 0 {
				continue
			}
			text := formatWatchLines(lines, dropped)
			last = text

			hit, confidence, err := decideWatch(ctx, spec, lines)
			if err != nil {
				if ctx.Err() != nil {
					continue
				}
				failures++
				if failures >= configs.WATCH_MAX_DECIDE_FAILURES {
					return reports, last, err
				}
				continue
			}
			failures = 0
			if hit {
				reports++
				report(fmt.Sprintf("condition met at %s (confidence %.2f, report #%d); still running\n%s", time.Now().Format("15:04:05"), confidence, reports, text))
			}
		}
	}
}

type watchAnswer struct {
	Answers map[string]struct {
		Choice     string  `json:"choice"`
		Confidence float64 `json:"confidence"`
	} `json:"answers"`
}

func decideWatch(ctx context.Context, spec watchSpec, lines []string) (bool, float64, error) {
	body := map[string]any{
		"model": config.TypesafeModel,
		"state": map[string]any{
			"condition": spec.prompt,
			"logs":      lines,
		},
		"questions": map[string]any{
			"trigger": map[string]any{
				"type": "choice",
				"instructions": map[string]any{
					"question": "Do `logs` satisfy `condition`?",
					"focus":    "Judge only the lines in `logs`; each line is one stream message or one line of command output, prefixed with its arrival time.",
				},
				"criteria": map[string]any{
					"yes": "At least one line in `logs`, or the lines taken together, meet `condition`.",
					"no":  "Nothing in `logs` meets `condition`.",
				},
			},
		},
	}

	callCtx, cancel := context.WithTimeout(ctx, configs.TIMEOUT_DISPATCH_CALL)
	defer cancel()
	result, _, err := go_pkg_http.POST[watchAnswer](callCtx, nil, config.TypesafeEndpoint, map[string]string{
		"Authorization": "Bearer " + spec.apiKey,
	}, body, "json")
	if err != nil {
		return false, 0, utilsmsg.Wrap(utilsmsg.Upstream, "go_pkg_http.POST", "jev", err)
	}
	answer, ok := result.Answers["trigger"]
	if !ok {
		return false, 0, utilsmsg.New(utilsmsg.Upstream, "jev", "response has no trigger answer")
	}
	switch answer.Choice {
	case "yes":
		return true, answer.Confidence, nil
	case "no":
		return false, answer.Confidence, nil
	}
	return false, 0, utilsmsg.New(utilsmsg.Upstream, "jev", "invalid choice %q", answer.Choice)
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
	for b.size > b.limit && len(b.lines) > 1 {
		b.size -= len(b.lines[0])
		b.lines = b.lines[1:]
		b.dropped++
	}
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

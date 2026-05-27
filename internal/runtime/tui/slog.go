package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

var sseDataPrefix = []byte("data: ")

type Log struct {
	Source string
	Level  string
	Time   time.Time
	Msg    string
	Attrs  []slog.Attr
}

type tuiSlogHandler struct{}

func (h *tuiSlogHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= slog.LevelInfo
}

func (h *tuiSlogHandler) Handle(_ context.Context, r slog.Record) error {
	if isNoisySlog(r) {
		return nil
	}
	entry := Log{
		Source: "tui",
		Level:  levelLabel(r.Level),
		Time:   r.Time,
		Msg:    r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		entry.Attrs = append(entry.Attrs, a)
		return true
	})
	// * async send: handler may be invoked from bubbletea event loop (Update path)
	// * synchronous prog.Send would block on msgs channel and deadlock the loop
	go send(entry)
	return nil
}

func isNoisySlog(r slog.Record) bool {
	noisy := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "err" && strings.Contains(fmt.Sprintf("%v", a.Value.Any()), "unexpected end of JSON input") {
			noisy = true
			return false
		}
		return true
	})
	return noisy
}

func (h *tuiSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *tuiSlogHandler) WithGroup(_ string) slog.Handler      { return h }

func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func levelLineStyle(l string) lipgloss.Style {
	switch l {
	case "ERROR":
		return errorStyle
	case "WARN":
		return warnStyle
	}
	return hintStyle
}

func renderLogLine(e Log) string {
	body := e.Msg
	for _, a := range e.Attrs {
		body += " " + a.Key + "=" + fmt.Sprintf("%v", a.Value.Any())
	}
	body = strings.TrimSpace(body)
	if strings.HasPrefix(e.Msg, "Telegram Verification Code") {
		code := extractField(body, "code=")
		username := extractField(body, "name=")
		line := fmt.Sprintf("$ Telegram Verification Code: %s (%s)", code, username)
		return systemStyle.Render(line) + "\n"
	}
	if strings.HasPrefix(e.Msg, "Discord Verification Code") {
		code := extractField(body, "code=")
		username := extractField(body, "name=")
		line := fmt.Sprintf("$ Discord Verification Code: %s (%s)", code, username)
		return systemStyle.Render(line) + "\n"
	}
	line := "$ [" + e.Source + "] " + body + " - " + e.Time.Format("15:04:05")
	return levelLineStyle(e.Level).Render(line) + "\n"
}

func extractField(s, key string) string {
	_, rest, ok := strings.Cut(s, key)
	if !ok {
		return ""
	}
	val, _, _ := strings.Cut(rest, " ")
	return val
}

type sessionSubMgr struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	current string
}

var subMgr = &sessionSubMgr{}

func installSlogTUI(ctx context.Context) func() {
	prev := slog.Default()
	slog.SetDefault(slog.New(&tuiSlogHandler{}))
	subMgrParentCtx = ctx
	daemonCtx, daemonCancel := context.WithCancel(ctx)
	go subscribeSessionEvents(daemonCtx, "daemon")

	return func() {
		daemonCancel()
		subMgr.Stop()
		slog.SetDefault(prev)
	}
}

var subMgrParentCtx = context.Background()

// * unsubscribe self tui logs
func subscribeSessionLog(sessionID string) {
	_ = sessionID
	subMgr.Switch(subMgrParentCtx, "")
}

func (m *sessionSubMgr) Switch(parent context.Context, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID == m.current && m.cancel != nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.current = sessionID
	if sessionID == "" {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	go subscribeSessionEvents(ctx, sessionID)
}

func (m *sessionSubMgr) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.current = ""
}

func subscribeSessionEvents(ctx context.Context, sessionID string) {
	url := daemonBaseURL() + "/v1/session/" + sessionID + "/log"
	client := &http.Client{}
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			sleepBackoff(ctx, &backoff)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			sleepBackoff(ctx, &backoff)
			continue
		}
		backoff = time.Second // 連到後重置

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			buf := scanner.Bytes()
			if !bytes.HasPrefix(buf, sseDataPrefix) {
				continue
			}
			var ev agentTypes.Event
			if err := json.Unmarshal(buf[len(sseDataPrefix):], &ev); err != nil {
				continue
			}
			if ev.Type == agentTypes.EventDaemonLog {
				send(Log{
					Source: "daemon",
					Level:  ev.Source,
					Time:   time.Now(),
					Msg:    ev.Text,
				})
				continue
			}
			send(agentEvent{event: ev})
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			slog.Warn("SSE scanner",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
		resp.Body.Close()
	}
}

func sleepBackoff(ctx context.Context, backoff *time.Duration) {
	select {
	case <-time.After(*backoff):
	case <-ctx.Done():
	}
	*backoff *= 2
	if *backoff > 30*time.Second {
		*backoff = 30 * time.Second
	}
}

package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

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
	send(entry)
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
	if l == "ERROR" {
		return errorStyle
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
		username := extractField(body, "username=")
		line := fmt.Sprintf("$ Telegram Verification Code: %s (%s)", code, username)
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

func installSlogTUI(ctx context.Context) func() {
	prev := slog.Default()
	slog.SetDefault(slog.New(&tuiSlogHandler{}))

	tailCtx, cancel := context.WithCancel(ctx)
	go newDaemonLogTailer(tailCtx)

	return func() {
		cancel()
		slog.SetDefault(prev)
	}
}

func newDaemonLogTailer(ctx context.Context) {
	path := filepath.Join(filesystem.AgenvoyDir, "daemon.log")
	dir := filepath.Dir(path)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer w.Close()

	if err := w.Add(dir); err != nil {
		return
	}
	if err := w.Add(path); err == nil {
		defer w.Remove(path)
	}

	lastSize := fileSize(path)
	poll := time.NewTicker(500 * time.Millisecond)
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if filepath.Base(ev.Name) != "daemon.log" {
				continue
			}
			if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
				_ = w.Remove(path)
				_ = w.Add(path)
				lastSize = fileSize(path)
				continue
			}
			if ev.Has(fsnotify.Create) {
				_ = w.Add(path)
			}
			lastSize = drainDaemonLog(path, lastSize)
		case <-w.Errors:
		case <-poll.C:
			lastSize = drainDaemonLog(path, lastSize)
		}
	}
}

func drainDaemonLog(path string, lastSize int64) int64 {
	current := fileSize(path)
	if current <= lastSize {
		return current
	}
	file, err := os.Open(path)
	if err != nil {
		return lastSize
	}
	defer file.Close()
	if _, err := file.Seek(lastSize, 0); err != nil {
		return current
	}
	scanner := bufio.NewScanner(io.LimitReader(file, current-lastSize))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		raw := strings.TrimRight(scanner.Text(), "\r")
		if raw == "" {
			continue
		}
		entry, ok := parseDaemonLog(raw)
		if !ok {
			continue
		}
		send(entry)
	}
	return current
}

func parseDaemonLog(raw string) (Log, bool) {
	entry := Log{Source: "daemon", Time: time.Now()}
	rest := strings.TrimSpace(raw)

	if len(rest) >= 19 {
		if t, err := time.Parse("2006/01/02 15:04:05", rest[:19]); err == nil {
			entry.Time = t
			rest = strings.TrimSpace(rest[19:])
		}
	}
	if strings.HasPrefix(rest, "time=") {
		if i := strings.Index(rest, " "); i > 5 {
			if t, err := time.Parse(time.RFC3339Nano, rest[5:i]); err == nil {
				entry.Time = t
			}
			rest = strings.TrimSpace(rest[i+1:])
		}
	}

	level := ""
	for _, lvl := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
		if strings.HasPrefix(rest, lvl+" ") {
			level = lvl
			rest = strings.TrimSpace(rest[len(lvl):])
			break
		}
		if strings.HasPrefix(rest, "level="+lvl) {
			level = lvl
			rest = strings.TrimSpace(rest[len("level=")+len(lvl):])
			break
		}
	}
	if level == "" {
		level = "INFO"
	}
	if level == "DEBUG" {
		return Log{}, false
	}
	entry.Level = level

	if after, ok := strings.CutPrefix(rest, "msg="); ok {
		rest = after
		if quoted, ok := strings.CutPrefix(rest, "\""); ok {
			if msgText, tail, ok := strings.Cut(quoted, "\""); ok {
				attrText := strings.TrimSpace(tail)
				if attrText != "" {
					rest = msgText + " " + attrText
				} else {
					rest = msgText
				}
			}
		}
	}

	entry.Msg = rest
	return entry, true
}

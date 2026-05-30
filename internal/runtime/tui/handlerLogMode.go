package tui

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type tailLine struct {
	line    string
	foreign bool
}

type initTailer struct{}

func (t TUI) restartTailer() TUI {
	if t.tailCancel != nil {
		t.tailCancel()
		t.tailCancel = nil
	}
	sid := strings.TrimSpace(t.currentSessionID)
	subscribeSessionLog(sid)
	if sid == "" {
		return t
	}
	ctx, cancel := context.WithCancel(t.ctx)
	t.tailCancel = cancel
	go newActionTailer(ctx, sid)
	return t
}

var foreignMarkStyle = lipgloss.NewStyle().Foreground(colWarn)

func foreignWrap(s string) string {
	if s == "" {
		return s
	}
	prefix := foreignMarkStyle.Render("▌ ")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			continue
		}
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func newActionTailer(ctx context.Context, sid string) {
	if strings.TrimSpace(sid) == "" {
		return
	}
	path := filesystem.ActionLogPath(sid)
	dir := filepath.Dir(path)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("tail watcher",
			slog.String("session", sid),
			slog.String("error", err.Error()))
		return
	}
	defer w.Close()

	if err := w.Add(dir); err != nil {
		slog.Warn("tail watch dir",
			slog.String("session", sid),
			slog.String("error", err.Error()))
		return
	}
	if err := w.Add(path); err == nil {
		defer w.Remove(path)
	}

	lastSize := fileSize(path)
	pollTicker := time.NewTicker(300 * time.Millisecond)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if filepath.Base(ev.Name) != "action.log" {
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
			lastSize = drainNew(path, lastSize)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Warn("tail watcher error",
				slog.String("session", sid),
				slog.String("error", err.Error()))
		case <-pollTicker.C:
			lastSize = drainNew(path, lastSize)
		}
	}
}

func drainNew(path string, lastSize int64) int64 {
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

	own := session.GetHash()
	scanner := bufio.NewScanner(io.LimitReader(file, current-lastSize))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		raw := scanner.Text()
		parsed, ok := parseActionLine(raw)
		if !ok {
			continue
		}
		if parsed.hash == own {
			continue
		}
		line := renderActionLine(parsed)
		if line == "" {
			continue
		}
		send(tailLine{
			line:    foreignWrap(line),
			foreign: true,
		})
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("scanner error in drainNew",
			slog.String("error", err.Error()))
	}
	return current
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

func readAllLines(path string) []string {
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return nil
	}
	var out []string
	for raw := range strings.SplitSeq(text, "\n") {
		if line := formatLog(raw); line != "" {
			out = append(out, line)
		}
	}
	return out
}

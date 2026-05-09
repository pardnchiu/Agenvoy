package tui

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type logLine struct {
	line string
}

type logHistory struct {
	lines []string
}

func (t TUI) logMode(on bool) (TUI, tea.Cmd) {
	if on {
		if t.currentSessionID == "" {
			return t, nil
		}
		t.mode = logMode

		ctx, cancel := context.WithCancel(t.ctx)
		t.logCancel = cancel
		go newLogListener(ctx, t.currentSessionID)
		return t, tea.Println("\n" + hintStyle.Render(fmt.Sprintf("⎯ log mode · sessions/%s/action.log", utils.ShortenSessionID(t.currentSessionID))))
	}
	if t.logCancel != nil {
		t.logCancel()
		t.logCancel = nil
	}
	t.mode = cliMode
	return t, nil
}

func newLogListener(ctx context.Context, sid string) {
	path := filepath.Join(filesystem.SessionsDir, sid, "action.log")

	existing := readAllLines(path)
	send(logHistory{lines: existing})

	w, err := fsnotify.NewWatcher()
	if err != nil {
		send(logLine{line: errorStyle.Render("[!] watcher: " + err.Error())})
		return
	}
	defer w.Close()

	dir := filepath.Dir(path)
	if err := w.Add(dir); err != nil {
		send(logLine{line: errorStyle.Render("[!] watch dir: " + err.Error())})
		return
	}

	lastSize := fileSize(path)
	pollTicker := time.NewTicker(1 * time.Second)
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
			lastSize = cutNew(path, lastSize)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			slog.Warn("log watcher error",
				slog.String("error", err.Error()))
		case <-pollTicker.C:
			lastSize = cutNew(path, lastSize)
		}
	}
}

func cutNew(path string, lastSize int64) int64 {
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

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := formatLog(scanner.Text())
		if line == "" {
			continue
		}
		send(logLine{line: line})
	}
	return current
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

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

package tui

import (
	"log/slog"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
)

const (
	historyLimit = 50
)

func loadInputHistory(sid string) []string {
	if sid == "" {
		return nil
	}

	text, err := go_pkg_filesystem.ReadText(filesystem.InputHistoryPath(sid))
	if err != nil {
		return nil
	}

	var results []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		results = append(results, strings.ReplaceAll(line, sessionLog.ActionNewlineMarker, "\n"))
	}
	return results
}

func (t TUI) recordInputHistory(content string) TUI {
	if content == "" {
		return t
	}
	switch strings.TrimSpace(content) {
	case "/exit", "/quit":
		t.inputHistoryIdx = -1
		return t
	}
	if n := len(t.inputHistory); n > 0 && t.inputHistory[n-1] == content {
		t.inputHistoryIdx = -1
		return t
	}
	t.inputHistory = append(t.inputHistory, content)
	if len(t.inputHistory) > historyLimit {
		t.inputHistory = t.inputHistory[len(t.inputHistory)-historyLimit:]
	}
	t.inputHistoryIdx = -1

	if t.currentSessionID == "" {
		return t
	}

	var sb strings.Builder
	for _, entry := range t.inputHistory {
		sb.WriteString(strings.ReplaceAll(entry, "\n", sessionLog.ActionNewlineMarker))
		sb.WriteString("\n")
	}
	if err := go_pkg_filesystem.WriteFile(filesystem.InputHistoryPath(t.currentSessionID), sb.String(), 0644); err != nil {
		slog.Warn("go_pkg_filesystem.WriteFile",
			slog.String("session", t.currentSessionID),
			slog.String("error", err.Error()))
	}
	return t
}

func (t TUI) clickUp() (TUI, bool) {
	if len(t.inputHistory) == 0 {
		return t, false
	}

	next := t.inputHistoryIdx
	switch {
	case next < 0:
		next = len(t.inputHistory) - 1
	case next == 0:
		return t, true
	default:
		next--
	}

	t.inputHistoryIdx = next
	t.textarea.SetValue(t.inputHistory[next])
	t.textarea.CursorEnd()
	t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
	t.selector = nil
	return t, true
}

func (t TUI) clickDown() (TUI, bool) {
	if t.inputHistoryIdx < 0 {
		if t.textarea.Value() == "" {
			return t, false
		}

		t.textarea.SetValue("")
		t.textarea.SetHeight(1)
		t.selector = nil
		return t, true
	}

	next := t.inputHistoryIdx + 1
	if next >= len(t.inputHistory) {
		t.inputHistoryIdx = -1
		t.textarea.SetValue("")
		t.textarea.SetHeight(1)
		t.selector = nil
		return t, true
	}

	t.inputHistoryIdx = next
	t.textarea.SetValue(t.inputHistory[next])
	t.textarea.CursorEnd()
	t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
	t.selector = nil
	return t, true
}

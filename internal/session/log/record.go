package sessionLog

import (
	"log/slog"
	"os"
	"strings"
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

const (
	ActionNewlineMarker = "\x1F"
)

var (
	assistantBody   = map[string]*strings.Builder{}
	assistantBodyMu sync.Mutex
	actionLogMu     sync.Mutex
)

func Append(sessionID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	appendAction(sessionID, withTimestamp("user", flatten(text)))
}

func Record(sessionID string, event agentTypes.Event) {
	switch event.Type {
	case agentTypes.EventText:
		appendAssistant(sessionID, event)
		return
	case agentTypes.EventTextDone:
		flushAssistant(sessionID, event)
		return
	}

	flushAssistant(sessionID, event)

	line := formatActionEvent(event)
	if line == "" {
		return
	}
	appendAction(sessionID, line)
}

func appendAssistant(sessionID string, event agentTypes.Event) {
	text := strings.TrimSpace(event.Text)
	if text == "" || sessionID == "" {
		return
	}

	key := sessionID + "\x00" + event.Source
	assistantBodyMu.Lock()
	defer assistantBodyMu.Unlock()

	sb, ok := assistantBody[key]
	if !ok {
		sb = &strings.Builder{}
		assistantBody[key] = sb
	}
	if sb.Len() > 0 {
		sb.WriteByte('\n')
	}
	sb.WriteString(text)
}

func flushAssistant(sessionID string, event agentTypes.Event) {
	if sessionID == "" {
		return
	}

	key := sessionID + "\x00" + event.Source
	assistantBodyMu.Lock()
	sb, ok := assistantBody[key]
	if !ok || sb.Len() == 0 {
		assistantBodyMu.Unlock()
		return
	}
	full := sb.String()
	delete(assistantBody, key)
	assistantBodyMu.Unlock()

	line := withTimestamp("assistant", flatten(full))
	appendAction(sessionID, line)
}

func appendAction(sessionID, line string) {
	if sessionID == "" || line == "" {
		return
	}
	actionLogMu.Lock()
	defer actionLogMu.Unlock()

	if !go_pkg_filesystem_reader.Exists(filesystem.SessionDir(sessionID)) {
		return
	}

	path := filesystem.ActionLogPath(sessionID)
	if err := go_pkg_filesystem.AppendText(path, line+"\n"); err != nil {
		slog.Warn("AppendText",
			slog.String("file", path),
			slog.String("error", err.Error()))
		return
	}

	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxActionLogSize {
		return
	}
	trim(path)
}

func trim(path string) {
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem ReadText",
			slog.String("file", path),
			slog.String("error", err.Error()))
		return
	}

	data := []byte(text)
	if int64(len(data)) <= maxActionLogSize {
		return
	}

	cut := max(len(data)-trimTargetSize, 0)
	for cut < len(data) && data[cut] != '\n' {
		cut++
	}
	if cut < len(data) {
		cut++
	}
	if cut >= len(data) {
		if err := go_pkg_filesystem.WriteFile(path, "", 0644); err != nil {
			slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteFile",
				slog.String("file", path),
				slog.String("error", err.Error()))
		}
		return
	}
	if err := go_pkg_filesystem.WriteFile(path, string(data[cut:]), 0644); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem WriteFile",
			slog.String("file", path),
			slog.String("error", err.Error()))
	}
}

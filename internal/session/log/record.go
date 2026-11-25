package sessionLog

import (
	"log/slog"
	"os"
	"strings"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	ActionNewlineMarker = "\x1F"
)

var (
	assistantBody   = map[string]*strings.Builder{}
	assistantBodyMu sync.Mutex
	actionLogMu     sync.Mutex
)

func Append(sessionID, windowHash, str string) {
	str = strings.TrimSpace(str)
	if str == "" {
		return
	}
	appendAction(sessionID, withTimestamp(windowHash, "user", "", flatten(str)))
}

func Steer(sessionID, windowHash, str string) {
	str = strings.TrimSpace(str)
	if str == "" {
		return
	}
	flushAssistant(sessionID, agenttypes.Event{WindowHash: windowHash})
	appendAction(sessionID, withTimestamp(windowHash, "steer", "", flatten(str)))
}

func Record(sessionID string, event agenttypes.Event) {
	switch event.Type {
	case agenttypes.EventTextDelta:
		return
	case agenttypes.EventText:
		appendAssistant(sessionID, event)
		return
	case agenttypes.EventTextDone:
		flushAssistant(sessionID, event)
		return
	}

	flushAssistant(sessionID, event)

	if event.Type == agenttypes.EventCanceled && strings.TrimSpace(event.Text) != "" {
		appendAction(sessionID, withTimestamp(event.WindowHash, "hook", event.TaskHash, flatten(event.Text)))
	}

	line := formatActionEvent(event)
	if line == "" {
		return
	}
	appendAction(sessionID, line)
}

func appendAssistant(sessionID string, event agenttypes.Event) {
	if sessionID == "" {
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
	sb.WriteString(event.Text)
	sb.WriteByte('\n')
}

func flushAssistant(sessionID string, event agenttypes.Event) {
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
	full := strings.TrimSpace(sb.String())
	delete(assistantBody, key)
	assistantBodyMu.Unlock()

	if full == "" {
		return
	}

	line := withTimestamp(event.WindowHash, "assistant", event.TaskHash, flatten(full))
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
	if err != nil || info.Size() <= configs.MAX_DOCUMENT_BYTES {
		return
	}
	trim(path)
}

func trim(path string) {
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		slog.Warn("go_pkg_filesystem.ReadText",
			slog.String("file", path),
			slog.String("error", err.Error()))
		return
	}

	data := []byte(text)
	if len(data) <= configs.MAX_DOCUMENT_BYTES {
		return
	}

	cut := max(len(data)-configs.MAX_DOCUMENT_BYTES*3/4, 0)
	for cut < len(data) && data[cut] != '\n' {
		cut++
	}
	if cut < len(data) {
		cut++
	}
	if cut >= len(data) {
		if err := go_pkg_filesystem.WriteFile(path, "", 0644); err != nil {
			slog.Warn("go_pkg_filesystem.WriteFile",
				slog.String("file", path),
				slog.String("error", err.Error()))
		}
		return
	}
	if err := go_pkg_filesystem.WriteFile(path, string(data[cut:]), 0644); err != nil {
		slog.Warn("go_pkg_filesystem.WriteFile",
			slog.String("file", path),
			slog.String("error", err.Error()))
	}
}

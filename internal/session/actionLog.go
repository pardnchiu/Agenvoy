package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const (
	maxActionLogSize  = 1 << 20
	trimTargetSize    = 768 << 10
	actionFieldMaxLen = 256
)

var actionLogMu sync.Mutex

func Record(sessionID string, event agentTypes.Event) {
	line := formatActionEvent(event)
	if line == "" {
		return
	}
	appendActionLine(sessionID, line)
}

func AppendActionUserInput(sessionID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	appendActionLine(sessionID, formatActionLine("user", truncateActionField(text)))
}

func GeadRecord(sessionID string, n int) []string {
	if sessionID == "" || n <= 0 {
		return nil
	}
	actionLogMu.Lock()
	defer actionLogMu.Unlock()

	path := filepath.Join(filesystem.SessionsDir, sessionID, "action.log")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	const chunkSize = 8 << 10
	var (
		buf      []byte
		pos      = info.Size()
		newlines = 0
	)
	for pos > 0 && newlines <= n {
		readSize := int64(chunkSize)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, pos); err != nil {
			return nil
		}
		for _, b := range chunk {
			if b == '\n' {
				newlines++
			}
		}
		buf = append(chunk, buf...)
	}

	text := strings.TrimRight(string(buf), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func formatActionEvent(ev agentTypes.Event) string {
	switch ev.Type {
	case agentTypes.EventText:
		text := strings.TrimSpace(ev.Text)
		if text == "" {
			return ""
		}
		return formatActionLine("assistant", truncateActionField(text))
	case agentTypes.EventToolCall:
		body := ev.ToolName
		if ev.ToolArgs != "" {
			body = fmt.Sprintf("%s %s", ev.ToolName, truncateActionField(ev.ToolArgs))
		}
		return formatActionLine("tool_call", body)
	case agentTypes.EventToolResult:
		status := "ok"
		if ev.Err != nil {
			status = "err"
		}
		return formatActionLine("tool_result", fmt.Sprintf("%s %s", ev.ToolName, status))
	case agentTypes.EventToolSkipped:
		return formatActionLine("tool_skipped", ev.ToolName)
	case agentTypes.EventToolConfirm:
		return formatActionLine("tool_confirm", ev.ToolName)
	case agentTypes.EventExecError, agentTypes.EventError:
		if ev.Err != nil {
			return formatActionLine("error", truncateActionField(ev.Err.Error()))
		}
		if ev.Text != "" {
			return formatActionLine("error", truncateActionField(ev.Text))
		}
		return ""
	case agentTypes.EventDone:
		return formatActionLine("done", ev.Model)
	}
	return ""
}

func formatActionLine(kind, body string) string {
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	return fmt.Sprintf("[%s][%s] %s", ts, kind, body)
}

func truncateActionField(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	if len(s) <= actionFieldMaxLen {
		return s
	}
	return s[:actionFieldMaxLen] + "…[truncated]"
}

func appendActionLine(sessionID, line string) {
	if sessionID == "" || line == "" {
		return
	}
	actionLogMu.Lock()
	defer actionLogMu.Unlock()

	dir := filepath.Join(filesystem.SessionsDir, sessionID)
	if _, err := os.Stat(dir); err != nil {
		return
	}
	path := filepath.Join(dir, "action.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Warn("appendActionLine open",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		f.Close()
		slog.Warn("appendActionLine write",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}
	info, statErr := f.Stat()
	f.Close()
	if statErr != nil || info.Size() <= maxActionLogSize {
		return
	}
	trimActionLog(path)
}

func trimActionLog(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("trimActionLog read",
			slog.String("error", err.Error()))
		return
	}
	if int64(len(data)) <= maxActionLogSize {
		return
	}
	cut := len(data) - trimTargetSize
	if cut < 0 {
		cut = 0
	}
	for cut < len(data) && data[cut] != '\n' {
		cut++
	}
	if cut < len(data) {
		cut++
	}
	if cut >= len(data) {
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			slog.Warn("trimActionLog wipe",
				slog.String("error", err.Error()))
		}
		return
	}
	if err := os.WriteFile(path, data[cut:], 0644); err != nil {
		slog.Warn("trimActionLog write",
			slog.String("error", err.Error()))
	}
}

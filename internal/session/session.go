package session

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

func New(prefix string) (string, error) {
	uuid := go_pkg_utils.UUID()
	if uuid == "" {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID: no UUID generated")
	}

	sessionID := prefix + uuid
	if err := configBot.Save(sessionID, "", "", false); err != nil {
		slog.Warn("configBot Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
	return sessionID, nil
}

func GetSessionID(name string) string {
	if name == "" {
		return ""
	}

	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return ""
	}

	for _, dir := range dirs {
		sid := dir.Name
		if strings.HasPrefix(sid, "temp-") {
			continue
		}

		botName, _ := configBot.Get(sid)
		if botName == "" {
			continue
		}
		if botName == name {
			return sid
		}
	}
	return ""
}

func SaveToToolCall(sessionID, content string) {
	now := time.Now()
	date := now.Format("2006-01-02")
	filename := fmt.Sprintf("%s.json", now.Format("2006-01-02-15-04-05"))
	toolActionsPath := filepath.Join(filesystem.SessionDir(sessionID), "tool_calls", date, filename)
	if err := go_pkg_filesystem.WriteFile(toolActionsPath, content, 0644); err != nil {
		slog.Warn("WriteFile",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

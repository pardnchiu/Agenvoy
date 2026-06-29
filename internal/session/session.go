package session

import (
	"fmt"
	"log/slog"
	"strings"

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

func FindIdleTemp() string {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return ""
	}
	for _, dir := range dirs {
		sid := dir.Name
		if !strings.HasPrefix(sid, "temp-") {
			continue
		}
		if ClaimIdle(sid) {
			return sid
		}
	}
	return ""
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

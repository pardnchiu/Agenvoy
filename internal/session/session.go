package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/pardnchiu/agenvoy/internal/utils"
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

func CreateSession(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand.Read: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)

	uuid := h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
	sessionID := prefix + uuid
	if err := go_pkg_filesystem.CheckDir(filepath.Join(filesystem.SessionsDir, sessionID), true); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	configBot.Save(sessionID, sessionID, "", false)
	return sessionID, nil
}

func GetLineSession(sourceType, userID, groupID, roomID string) (string, error) {
	var key, target string
	switch {
	case groupID != "":
		key = "ln_g_" + groupID
		target = groupID
	case roomID != "":
		key = "ln_r_" + roomID
		target = roomID
	default:
		key = "ln_u_" + userID
		target = userID
	}
	config := map[string]string{
		"line_target":      target,
		"line_source_type": sourceType,
	}
	sum := sha256.Sum256([]byte(key))

	sessionID := "ln-" + hex.EncodeToString(sum[:])
	sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
	configPath := filepath.Join(sessionDir, "config.json")

	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
		}
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON: %w", err)
		}
	}

	botName := configBot.FormatName(utils.LookupChatName(filesystem.LineAuthPath, target))
	if err := configBot.Save(sessionID, botName, "", false); err != nil {
		slog.Warn("configBot Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
	if botName != "" {
		configBot.ReplaceDefault(sessionID, botName)
	}
	return sessionID, nil
}

func GetLineTarget(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filepath.Join(filesystem.SessionsDir, sessionID, "config.json")
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON: %w", err)
	}
	return config["line_target"], nil
}

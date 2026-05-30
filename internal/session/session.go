package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionBot "github.com/pardnchiu/agenvoy/internal/session/bot"
)

var (
	historyMuMap sync.Map
)

func AppendHistory(sessionID string, delta []agentTypes.Message) error {
	if sessionID == "" || len(delta) == 0 {
		return nil
	}

	mu, _ := historyMuMap.LoadOrStore(sessionID, &sync.Mutex{})
	lock := mu.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	sessionDir := filesystem.SessionDir(sessionID)
	if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	historyPath := filepath.Join(sessionDir, "history.json")

	latest, err := go_pkg_filesystem.ReadJSON[[]agentTypes.Message](historyPath)
	if err != nil {
		latest = nil // first turn / missing file: start fresh
	}
	latest = append(latest, delta...)

	data, err := json.Marshal(latest)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if err := go_pkg_filesystem.WriteFile(historyPath, string(data), 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile: %w", err)
	}
	return nil
}

func SaveToToolCall(sessionID, content string) {
	now := time.Now()
	date := now.Format("2006-01-02")
	toolCallsDir := filepath.Join(filesystem.SessionDir(sessionID), "tool_calls", date)
	if err := go_pkg_filesystem.CheckDir(toolCallsDir, true); err == nil {
		filename := fmt.Sprintf("%s.json", now.Format("2006-01-02-15-04-05"))
		toolActionsPath := filepath.Join(toolCallsDir, filename)
		if err := go_pkg_filesystem.WriteFile(toolActionsPath, content, 0644); err != nil {
			slog.Warn("WriteFile",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
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
	if err := go_pkg_filesystem.CheckDir(filesystem.SessionDir(sessionID), true); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	if err := sessionBot.Save(sessionID, "", "", false); err != nil {
		slog.Warn("sessionBot Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
	return sessionID, nil
}

func GetTelegramSession(chatID int64) (string, error) {
	key := fmt.Sprintf("tg_%d", chatID)
	config := map[string]string{
		"chat_id": fmt.Sprintf("%d", chatID),
	}
	sum := sha256.Sum256([]byte(key))

	sessionID := "tg-" + hex.EncodeToString(sum[:])
	sessionDir := filesystem.SessionDir(sessionID)
	configPath := filepath.Join(sessionDir, "config.json")

	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
		}
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON: %w", err)
		}
	}

	if err := sessionBot.Save(sessionID, "", "", false); err != nil {
		slog.Warn("sessionBot Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
	return sessionID, nil
}

func GetDiscordSession(guildID, channelID, userID string) (string, error) {
	if guildID == "" {
		guildID = "dm"
	}
	if channelID == "" {
		channelID = "ch"
	}
	var key string
	var config map[string]string
	if guildID == "dm" {
		key = fmt.Sprintf("%s_%s", channelID, userID)
		config = map[string]string{
			"channel_id": channelID,
			"user_id":    userID,
		}
	} else {
		key = fmt.Sprintf("%s_%s", guildID, channelID)
		config = map[string]string{
			"guild_id":   guildID,
			"channel_id": channelID,
		}
	}
	sum := sha256.Sum256([]byte(key))

	sessionID := "dc-" + hex.EncodeToString(sum[:])
	sessionDir := filesystem.SessionDir(sessionID)
	configPath := filepath.Join(sessionDir, "config.json")

	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
		}
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON: %w", err)
		}
	}

	if err := sessionBot.Save(sessionID, "", "", false); err != nil {
		slog.Warn("sessionBot Save",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
	return sessionID, nil
}

func GetChannelID(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filepath.Join(filesystem.SessionDir(sessionID), "config.json")
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON: %w", err)
	}
	return config["channel_id"], nil
}

func GetChatID(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filepath.Join(filesystem.SessionDir(sessionID), "config.json")
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON: %w", err)
	}
	return config["chat_id"], nil
}

func GetHistory(sessionID string) (old, max []agentTypes.Message) {
	historyPath := filepath.Join(filesystem.SessionDir(sessionID), "history.json")
	oldHistory, err := go_pkg_filesystem.ReadJSON[[]agentTypes.Message](historyPath)
	if err != nil {
		return nil, nil
	}

	maxHistory := oldHistory
	if len(oldHistory) > filesystem.MaxHistoryMessages {
		maxHistory = oldHistory[len(oldHistory)-filesystem.MaxHistoryMessages:]
	}
	return oldHistory, maxHistory
}

func Clean() {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "temp-") {
			continue
		}
		sessionDir := filesystem.SessionDir(entry.Name())
		if now.Sub(latestModTime(sessionDir)) > time.Hour {
			if err := os.RemoveAll(sessionDir); err != nil {
				slog.Warn("Clean",
					slog.String("session", entry.Name()),
					slog.String("dir", entry.Name()),
					slog.String("error", err.Error()))
			}
		}
	}
}

func latestModTime(dir string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
		return nil
	})
	return latest
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

		botName, _ := sessionBot.Get(sid)
		if botName == "" {
			continue
		}
		if botName == name {
			return sid
		}
	}
	return ""
}

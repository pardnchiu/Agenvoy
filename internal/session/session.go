package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func SaveToToolCall(sessionID, content string) {
	now := time.Now()
	date := now.Format("2006-01-02")
	toolCallsDir := filepath.Join(filesystem.SessionsDir, sessionID, "tool_calls", date)
	if err := go_pkg_filesystem.CheckDir(toolCallsDir, true); err == nil {
		filename := fmt.Sprintf("%s.json", now.Format("2006-01-02-15-04-05"))
		toolActionsPath := filepath.Join(toolCallsDir, filename)
		if err := go_pkg_filesystem.WriteFile(toolActionsPath, content, 0644); err != nil {
			slog.Warn("WriteFile",
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
	if err := go_pkg_filesystem.CheckDir(filepath.Join(filesystem.SessionsDir, sessionID), true); err != nil {
		return "", fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}
	SaveBot(sessionID, sessionID, false)
	return sessionID, nil
}

func LockConfig() (func(), error) {
	// * lock file: kept on os.OpenFile because syscall.Flock needs the raw fd
	lockPath := filepath.Join(filesystem.AgenvoyDir, "config.json.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("os.OpenFile: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("syscall.Flock: %w", err)
	}

	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
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
	sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
	configPath := filepath.Join(sessionDir, "config.json")

	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
			return "", fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
		}
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("WriteJSON: %w", err)
		}
	}

	SaveBot(sessionID, sessionID, false)
	return sessionID, nil
}

func GetChannelID(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filepath.Join(filesystem.SessionsDir, sessionID, "config.json")
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("go_pkg_filesystem.ReadJSON: %w", err)
	}
	return config["channel_id"], nil
}

var MaxHistoryMessages = func() int {
	if n := go_pkg_utils.GetWithDefaultInt("MAX_HISTORY_MESSAGES", 16); n > 0 {
		return n
	}
	return 16
}()

func GetHistory(sessionID string) (old, max []agentTypes.Message) {
	historyPath := filepath.Join(filesystem.SessionsDir, sessionID, "history.json")
	oldHistory, err := go_pkg_filesystem.ReadJSON[[]agentTypes.Message](historyPath)
	if err != nil {
		return nil, nil
	}

	maxHistory := oldHistory
	if len(oldHistory) > MaxHistoryMessages {
		maxHistory = oldHistory[len(oldHistory)-MaxHistoryMessages:]
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
		sessionDir := filepath.Join(filesystem.SessionsDir, entry.Name())
		if now.Sub(latestModTime(sessionDir)) > time.Hour {
			if err := os.RemoveAll(sessionDir); err != nil {
				slog.Warn("CleanupTempSessions",
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

func SaveHistory(sessionID, content string) error {
	sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
	if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
		return fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
	}

	historyPath := filepath.Join(sessionDir, "history.json")
	if err := go_pkg_filesystem.WriteFile(historyPath, content, 0644); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

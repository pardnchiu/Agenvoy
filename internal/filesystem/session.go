package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
)

func SaveToToolCall(sessionID, content string) {
	now := time.Now()
	date := now.Format("2006-01-02")
	toolCallsDir := filepath.Join(SessionsDir, sessionID, "tool_calls", date)
	if err := os.MkdirAll(toolCallsDir, 0755); err == nil {
		filename := fmt.Sprintf("%s.json", now.Format("2006-01-02-15-04-05"))
		toolActionsPath := filepath.Join(toolCallsDir, filename)
		if err := WriteFile(toolActionsPath, content, 0644); err != nil {
			slog.Warn("WriteFile",
				slog.String("error", err.Error()))
		}
	}
}

func SaveHistory(sessionID, content string) error {
	sessionDir := filepath.Join(SessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	historyPath := filepath.Join(sessionDir, "history.json")
	if err := WriteFile(historyPath, content, 0644); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

func GetDiscordSessionID(guildID, channelID, userID string) (string, error) {
	if guildID == "" {
		guildID = "dm"
	}
	if channelID == "" {
		channelID = "ch"
	}
	key := fmt.Sprintf("%s_%s_%s", guildID, channelID, userID)
	sum := sha256.Sum256([]byte(key))

	sessionID := hex.EncodeToString(sum[:])
	sessionDir := filepath.Join(SessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll: %w", err)
	}

	configData, err := json.Marshal(map[string]string{
		"guild_id":   guildID,
		"channel_id": channelID,
		"user_id":    userID,
	})
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	configPath := filepath.Join(sessionDir, "config.json")
	if err := WriteFile(configPath, string(configData), 0644); err != nil {
		return "", fmt.Errorf("WriteFile: %w", err)
	}

	return sessionID, nil
}

func GetHistory(sessionID string) []byte {
	sessionDir := filepath.Join(SessionsDir, sessionID)
	historyPath := filepath.Join(sessionDir, "history.json")

	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil
	}
	return data
}

func GetSummary(sessionID string) string {
	sessionDir := filepath.Join(SessionsDir, sessionID)
	summaryPath := filepath.Join(sessionDir, "summary.json")
	bytes, err := os.ReadFile(summaryPath)
	if err != nil {
		return ""
	}
	summary := strings.NewReplacer(
		"{{.Summary}}", string(bytes),
	).Replace(strings.TrimSpace(configs.SummaryPrompt))
	return summary
}

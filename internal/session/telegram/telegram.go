package sessionTelegram

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func New(chatID int64) (string, error) {
	key := fmt.Sprintf("tg_%d", chatID)
	config := map[string]string{
		"chat_id": fmt.Sprintf("%d", chatID),
	}
	sum := sha256.Sum256([]byte(key))

	sessionID := "tg-" + hex.EncodeToString(sum[:])
	configPath := filesystem.SessionConfigPath(sessionID)
	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON [%s]: %w", configPath, err)
		}
	}

	botName := configBot.FormatName(utils.LookupChatName(filesystem.TelegramAuthPath, strconv.FormatInt(chatID, 10)))
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

func GetChat(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filesystem.SessionConfigPath(sessionID)
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON [%s]: %w", configPath, err)
	}
	return config["chat_id"], nil
}

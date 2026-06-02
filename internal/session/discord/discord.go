package sessionDiscord

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func New(guildID, channelID, userID string) (string, error) {
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
	configPath := filesystem.SessionConfigPath(sessionID)
	if !go_pkg_filesystem_reader.Exists(configPath) {
		if err := go_pkg_filesystem.WriteJSON(configPath, config, false); err != nil {
			return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteJSON [%s]: %w", configPath, err)
		}
	}

	botName := configBot.FormatName(utils.LookupChatName(filesystem.DiscordAuthPath, channelID))
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

func GetChannel(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("sessionID is required")
	}

	configPath := filesystem.SessionConfigPath(sessionID)
	config, err := go_pkg_filesystem.ReadJSON[map[string]string](configPath)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadJSON [%s]: %w", configPath, err)
	}
	return config["channel_id"], nil
}

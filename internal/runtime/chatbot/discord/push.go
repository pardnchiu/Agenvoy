package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/core/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessiondiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func PushDiscordResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	str := strings.TrimSpace(payload.Text)
	if id == "" || str == "" || !strings.HasPrefix(id, "dc-") {
		return
	}

	channelID, err := sessiondiscord.GetChannel(id)
	if err != nil {
		slog.Debug("session.GetChannelID",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}
	if channelID == "" {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("go_pkg_filesystem_keychain.Get",
			slog.String("session", id),
			slog.String("key", Key))
		return
	}
	client, err := go_bot_discord.New(token)
	if err != nil {
		slog.Warn("go_bot_discord.New",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}

	chanName := utils.LookupChatName(filesystem.DiscordAuthPath, channelID)
	cleanText, attachmentPaths := utils.ExtractFileMarkers(str)

	if strings.TrimSpace(cleanText) != "" {
		message := cleanText + chatbot.BuildPushFooter(chatbot.Discord, payload.Duration, payload.OutputElapsed, payload.Usage)
		if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
			quoted := strings.ReplaceAll(prefix, "\n", "\n> ")
			message = fmt.Sprintf("> %s\n%s", quoted, message)
		}
		for _, part := range chatbot.Chunk(chatbot.Discord, message) {
			if _, err := client.Send(ctx, channelID, "", part); err != nil {
				slog.Warn("go_bot_discord.Bot.Send",
					slog.String("session", id),
					slog.String("channel", chanName),
					slog.String("error", err.Error()))
				break
			}
		}
	}

	sendAttachments(ctx, client, channelID, chanName, "", attachmentPaths)
}

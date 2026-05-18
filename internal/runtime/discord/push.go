package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func PushDiscordResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	text := strings.TrimSpace(payload.Text)
	if id == "" || text == "" || !strings.HasPrefix(id, "dc-") {
		return
	}

	channelID, err := sessionManager.GetChannelID(id)
	if err != nil {
		slog.Warn("github.com/pardnchiu/agenvoy/internal/session GetChannelID",
			slog.String("session_id", id),
			slog.String("error", err.Error()))
		return
	}
	if channelID == "" {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("session_id", id),
			slog.String("key", Key))
		return
	}
	client, err := go_bot_discord.New(token)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord New",
			slog.String("error", err.Error()))
		return
	}

	chanName := utils.LookupChatName(filesystem.DiscordAuthPath, channelID)
	cleanText, attachmentPaths := extractFileMarkers(text)

	if strings.TrimSpace(cleanText) != "" {
		message := cleanText + buildPushFooter(payload.Model, payload.Usage)
		if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
			quoted := strings.ReplaceAll(prefix, "\n", "\n> ")
			message = fmt.Sprintf("> %s\n%s", quoted, message)
		}
		if _, err := client.Send(ctx, channelID, "", message); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.Send",
				slog.String("channel", chanName),
				slog.String("error", err.Error()))
		}
	}

	sendAttachments(ctx, client, channelID, chanName, "", attachmentPaths)
}

func buildPushFooter(model string, usage *agentTypes.Usage) string {
	model = strings.TrimSpace(model)
	if model == "" && usage == nil {
		return ""
	}
	if _, after, ok := strings.Cut(model, "@"); ok {
		model = after
	}
	footer := model
	if usage != nil {
		if footer != "" {
			footer = fmt.Sprintf("%s | in:%s out:%s", footer, utils.FormatUsage(usage.Input), utils.FormatUsage(usage.Output))
		} else {
			footer = fmt.Sprintf("in:%s out:%s", utils.FormatUsage(usage.Input), utils.FormatUsage(usage.Output))
		}
	}
	return "\n-# ⎿ " + footer
}

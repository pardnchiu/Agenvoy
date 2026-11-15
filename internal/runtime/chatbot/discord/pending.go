package discord

import (
	"context"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessiondiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type discordTransport struct {
	bot *Bot
}

func newPendingListener(bot *Bot) *runtime.Listener[string, string] {
	return runtime.New[string, string](
		&discordTransport{bot: bot},
		"dc-",
		func(c string) string {
			return utils.LookupChatName(filesystem.DiscordAuthPath, c)
		},
		bot.client.Delete,
	)
}

func (t *discordTransport) LookupChatID(sessionID string) (string, error) {
	channelID, err := sessiondiscord.GetChannel(sessionID)
	if err != nil {
		return "", utilsmsg.Trace("GetChannelID", "", err)
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "", utilsmsg.New(utilsmsg.NotFound, "channelID", "empty for session %s", sessionID)
	}
	return channelID, nil
}

func (t *discordTransport) SendConfirm(ctx context.Context, channelID string, confirm utils.ConfirmMessage, explain string, options []string) (string, error) {
	msg, err := t.bot.client.SendSelect(ctx, channelID, "", utils.FormatDiscordConfirm(confirm, explain), options)
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func (t *discordTransport) SendAskText(ctx context.Context, channelID, header string, secret bool) (string, error) {
	msg, err := t.bot.client.SendInput(ctx, channelID, "", utils.FormatDiscordAskText(header, secret))
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func (t *discordTransport) SendAskSingle(ctx context.Context, channelID, header string, options []string) (string, error) {
	msg, err := t.bot.client.SendSelect(ctx, channelID, "", header, options)
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func (t *discordTransport) SendAskMulti(ctx context.Context, channelID, header string, options []string) (string, error) {
	msg, err := t.bot.client.SendMultiSelect(ctx, channelID, "", header, options)
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

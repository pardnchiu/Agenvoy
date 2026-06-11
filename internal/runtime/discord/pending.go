package discord

import (
	"context"
	"fmt"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionDiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/utils"
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
	channelID, err := sessionDiscord.GetChannel(sessionID)
	if err != nil {
		return "", fmt.Errorf("GetChannelID: %w", err)
	}
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "", fmt.Errorf("empty channelID for session %s", sessionID)
	}
	return channelID, nil
}

func (t *discordTransport) SendConfirm(ctx context.Context, channelID, toolName, toolArgs string, multiline bool) (string, error) {
	toolArgs = go_pkg_utils.TruncateString(toolArgs, 1024)
	var str string
	if multiline {
		str = fmt.Sprintf("Run `%s`?\n```\n%s\n```", toolName, toolArgs)
	} else {
		str = fmt.Sprintf("Run `%s`?\n`%s`", toolName, toolArgs)
	}
	msg, err := t.bot.client.SendSelect(ctx, channelID, "", str, runtime.ConfirmOptions())
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func (t *discordTransport) SendAskText(ctx context.Context, channelID, header string, secret bool) (string, error) {
	prompt := header
	if secret {
		prompt += "\n\n🔒 Your reply is captured via a private modal and is not logged."
	}
	msg, err := t.bot.client.SendInput(ctx, channelID, "", prompt)
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

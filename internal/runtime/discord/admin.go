package discord

import (
	"context"
	"fmt"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

func SendAdminCode(ctx context.Context, channelID, text string) error {
	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		return fmt.Errorf("discord token missing")
	}
	client, err := go_bot_discord.New(token)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-bot/discord New: %w", err)
	}
	if _, err := client.Send(ctx, strings.TrimSpace(channelID), "", text); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-bot/discord Bot.Send: %w", err)
	}
	return nil
}

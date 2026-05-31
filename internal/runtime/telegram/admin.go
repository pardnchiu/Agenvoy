package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

func SendAdminCode(ctx context.Context, chatID, str string) error {
	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		return fmt.Errorf("telegram token missing")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(chatID), 10, 64)
	if err != nil {
		return fmt.Errorf("parse chatID %q: %w", chatID, err)
	}
	client, err := go_bot_telegram.New(token)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-bot/telegram New: %w", err)
	}
	if _, err := client.Send(ctx, id, 0, html.EscapeString(str), go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-bot/telegram Bot.Send: %w", err)
	}
	return nil
}

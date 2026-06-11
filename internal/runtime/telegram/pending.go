package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionTelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
)

type telegramTransport struct {
	bot *Bot
}

func newPendingListener(bot *Bot) *runtime.Listener[int64, int] {
	return runtime.New[int64, int](
		&telegramTransport{bot: bot},
		"tg-",
		func(c int64) string {
			return utils.LookupChatName(filesystem.TelegramAuthPath, strconv.FormatInt(c, 10))
		},
		bot.client.Delete,
	)
}

func (t *telegramTransport) LookupChatID(sessionID string) (int64, error) {
	chatStr, err := sessionTelegram.GetChat(sessionID)
	if err != nil {
		return 0, fmt.Errorf("GetChatID: %w", err)
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse chatID %q: %w", chatStr, err)
	}
	return chatID, nil
}

func (t *telegramTransport) SendConfirm(ctx context.Context, chatID int64, toolName, toolArgs string, multiline bool) (int, error) {
	const limit = 3200
	var str string
	var modes []go_bot_telegram.MessageOption
	truncated := go_pkg_utils.TruncateString(toolArgs, limit)
	if truncated != toolArgs {
		str = fmt.Sprintf("Run %s?\n\n%s", toolName, truncated)
	} else {
		var body string
		if multiline {
			body = fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(toolArgs))
		} else {
			body = fmt.Sprintf("<code>%s</code>", html.EscapeString(toolArgs))
		}
		str = fmt.Sprintf("Run %s?\n\n%s", html.EscapeString(toolName), body)
		modes = append(modes, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
	}
	msg, err := t.bot.client.SendSelect(ctx, chatID, 0, str, runtime.ConfirmOptions(), modes...)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (t *telegramTransport) SendAskText(ctx context.Context, chatID int64, header string, secret bool) (int, error) {
	prompt := header
	if secret {
		prompt += "\n\n🔒 Your reply will be deleted from chat after capture."
	}
	msg, err := t.bot.client.SendInput(ctx, chatID, 0, prompt)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (t *telegramTransport) SendAskSingle(ctx context.Context, chatID int64, header string, options []string) (int, error) {
	msg, err := t.bot.client.SendSelect(ctx, chatID, 0, header, options)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (t *telegramTransport) SendAskMulti(ctx context.Context, chatID int64, header string, options []string) (int, error) {
	msg, err := t.bot.client.SendMultiSelect(ctx, chatID, 0, header, options)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

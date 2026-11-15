package telegram

import (
	"context"
	"strconv"
	"strings"

	go_bot_telegram "github.com/pardnchiu/go-bot/core/telegram"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessiontelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
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
	chatStr, err := sessiontelegram.GetChat(sessionID)
	if err != nil {
		return 0, utilsmsg.Trace("GetChatID", "", err)
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(chatStr), 10, 64)
	if err != nil {
		return 0, utilsmsg.New(utilsmsg.Invalid, "chatID", "parse %q: %w", chatStr, err)
	}
	return chatID, nil
}

func (t *telegramTransport) SendConfirm(ctx context.Context, chatID int64, confirm utils.ConfirmMessage, explain string, options []string) (int, error) {
	str, isHTML := utils.FormatTelegramConfirm(confirm, explain)
	var modes []go_bot_telegram.MessageOption
	if isHTML {
		modes = append(modes, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
	}
	msg, err := t.bot.client.SendSelect(ctx, chatID, 0, str, options, modes...)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

func (t *telegramTransport) SendAskText(ctx context.Context, chatID int64, header string, secret bool) (int, error) {
	msg, err := t.bot.client.SendInput(ctx, chatID, 0, utils.FormatTelegramAskText(header, secret))
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

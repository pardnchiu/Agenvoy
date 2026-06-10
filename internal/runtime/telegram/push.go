package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessionTelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var brTagRegex = regexp.MustCompile(`(?i)<br\s*/?>`)

func sanitizeHTML(s string) string {
	return brTagRegex.ReplaceAllString(s, "\n")
}

func PushTelegramResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	str := sanitizeHTML(strings.TrimSpace(payload.Text))
	if id == "" || str == "" || !strings.HasPrefix(id, "tg-") {
		return
	}

	chatIDStr, err := sessionTelegram.GetChat(id)
	if err != nil {
		slog.Warn("github.com/pardnchiu/agenvoy/internal/session GetChatID",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}
	if chatIDStr == "" {
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		slog.Warn("strconv.ParseInt",
			slog.String("session", id),
			slog.String("chat_id", chatIDStr),
			slog.String("error", err.Error()))
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("session", id),
			slog.String("key", Key))
		return
	}
	client, err := go_bot_telegram.New(token)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram New",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}

	chatName := utils.LookupChatName(filesystem.TelegramAuthPath, strconv.FormatInt(chatID, 10))
	cleanText, photoPaths, docPaths := extractFileMarkers(str)

	if strings.TrimSpace(cleanText) != "" {
		message := cleanText + chatbot.BuildPushFooter(chatbot.Telegram, payload.Duration, payload.Model, payload.Usage)
		if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
			message = fmt.Sprintf("<blockquote>%s</blockquote>\n\n%s", html.EscapeString(prefix), message)
		}
		for _, chunk := range chatbot.Chunk(chatbot.Telegram, chatbot.SanitizeTelegramHTML(message)) {
			if _, err := client.Send(ctx, chatID, 0, chunk, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Send",
					slog.String("session", id),
					slog.String("chat", chatName),
					slog.String("error", err.Error()))
				break
			}
		}
	}

	sendAttachments(ctx, chatID, chatName, photoPaths, docPaths)
}

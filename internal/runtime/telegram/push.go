package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func PushTelegramResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	text := strings.TrimSpace(payload.Text)
	if id == "" || text == "" || !strings.HasPrefix(id, "tg-") {
		return
	}

	chatIDStr, err := sessionManager.GetChatID(id)
	if err != nil {
		slog.Warn("github.com/pardnchiu/agenvoy/internal/session GetChatID",
			slog.String("session_id", id),
			slog.String("error", err.Error()))
		return
	}
	if chatIDStr == "" {
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		slog.Warn("strconv.ParseInt",
			slog.String("chat_id", chatIDStr),
			slog.String("error", err.Error()))
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("session_id", id),
			slog.String("key", Key))
		return
	}
	client, err := go_bot_telegram.New(token)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram New",
			slog.String("error", err.Error()))
		return
	}

	message := text + buildPushFooter(payload.Model, payload.Usage)
	if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
		message = fmt.Sprintf("<blockquote>%s</blockquote>\n\n%s", html.EscapeString(prefix), message)
	}
	if _, err := client.Send(ctx, chatID, 0, message, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Send",
			slog.Int64("chat", chatID),
			slog.String("error", err.Error()))
	}
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
			footer = fmt.Sprintf("%s | in:%s out:%s", footer, fmtUsage(usage.Input), fmtUsage(usage.Output))
		} else {
			footer = fmt.Sprintf("in:%s out:%s", fmtUsage(usage.Input), fmtUsage(usage.Output))
		}
	}
	return "\n\n<blockquote expandable>" + footer + "</blockquote>"
}

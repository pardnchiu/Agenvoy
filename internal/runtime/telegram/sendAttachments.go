package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const attachmentSendTimeout = 10 * time.Minute

func sendAttachments(ctx context.Context, chatID int64, chatName string, replyToID int, photoPaths, docPaths []string) {
	if len(photoPaths) == 0 && len(docPaths) == 0 {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("chat", chatName),
			slog.String("key", Key))
		return
	}

	client, err := go_bot_telegram.New(token,
		go_bot_telegram.WithHTTPClient(&http.Client{Timeout: attachmentSendTimeout}))
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram New",
			slog.String("chat", chatName),
			slog.String("error", err.Error()))
		return
	}

	notifyFailure := func(label, detail, errMsg string) {
		body := fmt.Sprintf("<code>%s</code>", html.EscapeString(errMsg))
		if detail != "" {
			body = fmt.Sprintf("<code>%s</code>: %s", html.EscapeString(detail), body)
		}
		text := fmt.Sprintf("⚠️ %s failed\n%s", label, body)
		if _, err := client.Send(ctx, chatID, replyToID, text, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	for start := 0; start < len(photoPaths); start += 10 {
		end := start + 10
		end = min(end, len(photoPaths))
		if _, err := client.SendPhoto(ctx, chatID, photoPaths[start:end]); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.SendPhoto",
				slog.String("chat", chatName),
				slog.Int("count", end-start),
				slog.String("error", err.Error()))
			notifyFailure("SendPhoto", strings.Join(photoPaths[start:end], ", "), err.Error())
		}
	}
	for _, path := range docPaths {
		if _, err := client.SendFile(ctx, chatID, go_bot_telegram.TypeDocument, path); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.SendFile",
				slog.String("chat", chatName),
				slog.String("path", path),
				slog.String("error", err.Error()))
			notifyFailure("SendFile", path, err.Error())
		}
	}
}

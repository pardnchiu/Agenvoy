package line

import (
	"context"
	"log/slog"
	"strings"

	go_bot_line "github.com/pardnchiu/go-bot/line"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func PushLineResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	text := strings.TrimSpace(payload.Text)
	if id == "" || text == "" || !strings.HasPrefix(id, "ln-") {
		return
	}

	target, err := sessionManager.GetLineTarget(id)
	if err != nil {
		slog.Warn("github.com/pardnchiu/agenvoy/internal/session GetLineTarget",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}
	if target == "" {
		return
	}

	secret := strings.TrimSpace(keychain.Get(SecretKey))
	token := strings.TrimSpace(keychain.Get(TokenKey))
	if secret == "" || token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("session", id),
			slog.String("secret_key", SecretKey),
			slog.String("token_key", TokenKey))
		return
	}

	client, err := go_bot_line.New(secret, token, filesystem.LinePort)
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/line New",
			slog.String("session", id),
			slog.String("error", err.Error()))
		return
	}

	cleanText, _ := utils.ExtractFileMarkers(text)
	message := strings.TrimSpace(cleanText)
	if message == "" {
		return
	}
	if footer := utils.FormatEventFooter(payload.Duration, payload.Model, payload.Usage); footer != "" {
		message = message + "\n\n" + footer
	}
	if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
		message = prefix + "\n\n" + message
	}

	for _, part := range chunk(message) {
		if _, err := client.Send(ctx, target, part); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send",
				slog.String("session", id),
				slog.String("source", target),
				slog.String("error", err.Error()))
			break
		}
	}
}

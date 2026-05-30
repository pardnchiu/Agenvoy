package line

import (
	"context"
	"fmt"
	"log/slog"

	go_bot_line "github.com/pardnchiu/go-bot/line"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
)

const (
	SecretKey = "LINE_SECRET"
	TokenKey  = "LINE_TOKEN"
)

type Bot struct {
	client    *go_bot_line.Bot
	cancel    context.CancelFunc
	fileGroup *fileGroupBuffer
}

func New() (*Bot, error) {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.LineEnabled {
		return nil, nil
	}
	secret := keychain.Get(SecretKey)
	token := keychain.Get(TokenKey)
	if secret == "" || token == "" {
		return nil, nil
	}

	client, err := go_bot_line.New(secret, token, filesystem.LinePort)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/line New: %w", err)
	}

	bot := &Bot{client: client, fileGroup: newFileGroupBuffer()}

	client.Reply(func(ctx context.Context, in go_bot_line.Input) string {
		key := sourceID(in)
		if key == "" {
			return ""
		}
		// * attachments (and a caption joining an open group) buffer until the
		// * burst goes silent, so multi-file sends collapse into one turn; plain
		// * text with no open group runs immediately.
		if bot.fileGroup.offer(bot, key, in, inputHasAttachment(in)) {
			return ""
		}
		bgCtx := context.WithoutCancel(ctx)
		go func() {
			if err := run(bgCtx, bot, in, []go_bot_line.Input{in}); err != nil {
				slog.Warn("run",
					slog.String("source", key),
					slog.String("error", err.Error()))
			}
		}()
		return ""
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/line Start: %w", err)
	}
	bot.cancel = cancel

	if name := client.Status().DisplayName; name != "" && cfg.LineUsername != name {
		cfg.LineUsername = name
		if err := session.Save(cfg); err != nil {
			slog.Warn("github.com/pardnchiu/agenvoy/internal/session Save",
				slog.String("error", err.Error()))
		}
	}

	return bot, nil
}

func Close(b *Bot) error {
	if b == nil || b.client == nil {
		return nil
	}
	if b.cancel != nil {
		b.cancel()
	}
	return b.client.Close()
}

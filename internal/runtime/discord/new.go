package discord

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/session"
)

const Key = "DISCORD_TOKEN"

type Bot struct {
	client *go_bot_discord.Bot
	cancel context.CancelFunc
}

var current atomic.Pointer[Bot]

func Current() *Bot {
	return current.Load()
}

func (b *Bot) Client() *go_bot_discord.Bot {
	if b == nil {
		return nil
	}
	return b.client
}

func New() (*Bot, error) {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.DiscordEnabled {
		return nil, nil
	}
	token := keychain.Get(Key)
	if token == "" {
		return nil, nil
	}

	client, err := go_bot_discord.New(token)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/discord New: %w", err)
	}

	bot := &Bot{client: client}

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/discord Start: %w", err)
	}
	bot.cancel = cancel
	current.Store(bot)

	username := client.Status().Username
	if cfg, err := session.Load(); err == nil && cfg != nil && cfg.DiscordUsername != username {
		cfg.DiscordUsername = username
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
	current.CompareAndSwap(b, nil)
	if b.cancel != nil {
		b.cancel()
	}
	return b.client.Close()
}

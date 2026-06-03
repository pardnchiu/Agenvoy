package discord

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

const Key = "DISCORD_TOKEN"

type Bot struct {
	client   *go_bot_discord.Bot
	cancel   context.CancelFunc
	listener *runtime.Listener[string, string]
}

var current atomic.Pointer[Bot]

func (b *Bot) Client() *go_bot_discord.Bot {
	if b == nil {
		return nil
	}
	return b.client
}

func New() (*Bot, error) {
	cfg, err := config.Load()
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

	client.Reply(func(ctx context.Context, in go_bot_discord.Input) string {
		if err := run(ctx, bot, in); err != nil {
			slog.Warn("run",
				slog.String("channel", channelName(in)),
				slog.String("error", err.Error()))
		}
		return ""
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/discord Start: %w", err)
	}
	bot.cancel = cancel
	bot.listener = newPendingListener(bot)
	runtime.RegisterResumeHandler("dc-", bot.resumeFromPending)
	current.Store(bot)

	username := client.Status().Username
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.DiscordUsername != username {
		cfg.DiscordUsername = username
		if err := config.Save(cfg); err != nil {
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
	if b.listener != nil {
		b.listener.Stop()
		b.listener = nil
	}
	if b.cancel != nil {
		b.cancel()
	}
	return b.client.Close()
}

var pending = utils.NewPendingRegistry[string, string]()

func authorizeChannel(in go_bot_discord.Input) error {
	path := filesystem.DiscordAuthPath
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	line := in.ChannelID
	if name := strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(channelName(in))); name != "" {
		line = in.ChannelID + "-" + name
	}
	if err := go_pkg_filesystem.AppendText(path, line+"\n"); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AppendText: %w", err)
	}
	return nil
}

package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/utils"
	"github.com/pardnchiu/go-bot/telegram"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const Key = "TELEGRAM_TOKEN"

type Bot struct {
	client    *telegram.Bot
	cancel    context.CancelFunc
	listener  *runtime.Listener[int64, int]
	fileGroup *fileGroupBuffer
}

var current atomic.Pointer[Bot]

func (b *Bot) Client() *telegram.Bot {
	if b == nil {
		return nil
	}
	return b.client
}

func New() (*Bot, error) {
	cfg, err := config.Load()
	if err != nil || cfg == nil || !cfg.TelegramEnabled {
		return nil, nil
	}
	token := keychain.Get(Key)
	if token == "" {
		return nil, nil
	}

	client, err := telegram.New(token,
		telegram.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}),
	)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/telegram New: %w", err)
	}

	bot := &Bot{client: client, fileGroup: newFileGroupBuffer()}

	client.Reply(func(ctx context.Context, in telegram.Input) string {
		if gid := fileGroupID(in); gid != "" {
			bot.fileGroup.add(bot, gid, in)
			return ""
		}
		if err := run(ctx, bot, in, []telegram.Input{in}); err != nil {
			slog.Warn("run",
				slog.String("chat", chatName(in)),
				slog.String("error", err.Error()))
		}
		return ""
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/telegram Start: %w", err)
	}
	bot.cancel = cancel
	bot.listener = newPendingListener(bot)
	runtime.RegisterResumeHandler("tg-", bot.resumeFromPending)
	current.Store(bot)

	username := client.Status().Username
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.TelegramUsername != username {
		cfg.TelegramUsername = username
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

var pending = utils.NewPendingRegistry[int64, int]()

func authorizeChat(in go_bot_telegram.Input) error {
	path := filesystem.TelegramAuthPath
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	line := strconv.FormatInt(in.ChatID, 10)
	if name := strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(chatName(in))); name != "" {
		line = line + "-" + name
	}
	if err := go_pkg_filesystem.AppendText(path, line+"\n"); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AppendText: %w", err)
	}
	return nil
}

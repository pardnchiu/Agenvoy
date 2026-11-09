package telegram

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	go_bot_telegram "github.com/pardnchiu/go-bot/core/telegram"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const Key = "TELEGRAM_TOKEN"

type Bot struct {
	client    *go_bot_telegram.Bot
	cancel    context.CancelFunc
	listener  *runtime.Listener[int64, int]
	fileGroup *fileGroupBuffer
}

var current atomic.Pointer[Bot]

func New() (*Bot, error) {
	cfg, err := config.Load()
	if err != nil || cfg == nil || !cfg.TelegramEnabled {
		return nil, nil
	}
	token := keychain.Get(Key)
	if token == "" {
		return nil, nil
	}

	client, err := go_bot_telegram.New(token,
		go_bot_telegram.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}),
	)
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.New", "", err)
	}

	bot := &Bot{client: client, fileGroup: newFileGroupBuffer()}

	client.Reply(func(ctx context.Context, in go_bot_telegram.Input) string {
		if gid := fileGroupID(in); gid != "" {
			bot.fileGroup.add(bot, gid, in)
			return ""
		}
		if err := run(ctx, bot, in, []go_bot_telegram.Input{in}); err != nil {
			slog.Warn("run",
				slog.String("chat", chatName(in)),
				slog.String("error", err.Error()))
		}
		return ""
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.Start", "", err)
	}
	bot.cancel = cancel
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := client.SetCommands(cmdCtx, []go_bot_telegram.Command{
		{Name: "new", Description: "Start a separate task instead of adding to the running one"},
	}); err != nil {
		slog.Warn("[Upstream] telegram commands: register failed",
			slog.String("error", err.Error()))
	}
	cmdCancel()
	bot.listener = newPendingListener(bot)
	runtime.RegisterResumeHandler("tg-", bot.resumeFromPending)
	current.Store(bot)

	username := client.Status().Username
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.TelegramUsername != username {
		cfg.TelegramUsername = username
		if err := config.Save(cfg); err != nil {
			slog.Debug("session.Save",
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
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", "", err)
	}
	line := strconv.FormatInt(in.ChatID, 10)
	if name := strings.TrimSpace(strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(chatName(in))); name != "" {
		line = line + "-" + name
	}
	if err := go_pkg_filesystem.AppendText(path, line+"\n"); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.AppendText", "", err)
	}
	return nil
}

package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	"github.com/pardnchiu/agenvoy/internal/session"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type TelegramAction struct {
	action string
}

type TelegramTokenSubmit struct {
	token string
}

type TelegramDone struct {
	action string
	err    error
}

func (t TUI) commandTelegram(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable":
			action := parts[1]
			return t, func() tea.Msg { return TelegramAction{action: action} }, true
		}
	}

	enabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		enabled = cfg.TelegramEnabled && keychain.Get(telegram.Key) != ""
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Telegram",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return TelegramAction{action: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openTelegramTokenPrompt() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Telegram Bot Token",
		subtitle: "from @BotFather · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return TelegramTokenSubmit{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func enableTelegram(token string) tea.Cmd {
	return func() tea.Msg {
		if token == "" {
			return TelegramDone{action: "enable", err: fmt.Errorf("token is required")}
		}
		username, err := verifyTelegram(token)
		if err != nil {
			return TelegramDone{action: "enable", err: err}
		}
		if err := keychain.Set(telegram.Key, token); err != nil {
			return TelegramDone{action: "enable", err: fmt.Errorf("keychain.Set: %w", err)}
		}
		cfg, err := session.Load()
		if err != nil {
			return TelegramDone{action: "enable", err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.TelegramEnabled = true
		cfg.TelegramUsername = username
		if err := session.Save(cfg); err != nil {
			return TelegramDone{action: "enable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return TelegramDone{action: "enable"}
	}
}

func disableTelegram() tea.Cmd {
	return func() tea.Msg {
		cfg, err := session.Load()
		if err != nil {
			return TelegramDone{action: "disable", err: fmt.Errorf("session.Load: %w", err)}
		}
		if !cfg.TelegramEnabled && keychain.Get(telegram.Key) == "" {
			return TelegramDone{action: "disable"}
		}
		if err := keychain.Delete(telegram.Key); err != nil {
			slog.Warn("keychain.Delete telegram token",
				slog.String("error", err.Error()))
		}
		cfg.TelegramEnabled = false
		if err := session.Save(cfg); err != nil {
			return TelegramDone{action: "disable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return TelegramDone{action: "disable"}
	}
}

func verifyTelegram(token string) (string, error) {
	client, err := go_bot_telegram.New(token)
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-bot/telegram New: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-bot/telegram Start: %w", err)
	}
	username := client.Status().Username
	_ = client.Close()
	return username, nil
}

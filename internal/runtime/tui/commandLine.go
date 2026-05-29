package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime/line"
	"github.com/pardnchiu/agenvoy/internal/session"
	go_bot_line "github.com/pardnchiu/go-bot/line"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type LineAction struct {
	action string
}

type LineSecretSubmit struct {
	secret string
}

type LineTokenSubmit struct {
	secret string
	token  string
}

type LineDone struct {
	action string
	err    error
}

func (t TUI) commandLine(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable":
			action := parts[1]
			return t, func() tea.Msg { return LineAction{action: action} }, true
		}
	}

	enabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		enabled = cfg.LineEnabled && keychain.Get(line.SecretKey) != "" && keychain.Get(line.TokenKey) != ""
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "LINE",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return LineAction{action: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openLineSecretPrompt() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "LINE Channel Secret",
		subtitle: "from LINE Developers Console · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return LineSecretSubmit{secret: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openLineTokenPrompt(secret string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "LINE Channel Access Token",
		subtitle: "long-lived token from LINE Developers Console · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return LineTokenSubmit{secret: secret, token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func enableLine(secret, token string) tea.Cmd {
	return func() tea.Msg {
		if secret == "" {
			return LineDone{action: "enable", err: fmt.Errorf("channel secret is required")}
		}
		if token == "" {
			return LineDone{action: "enable", err: fmt.Errorf("channel access token is required")}
		}
		name, err := verifyLine(secret, token)
		if err != nil {
			return LineDone{action: "enable", err: err}
		}
		if err := keychain.Set(line.SecretKey, secret); err != nil {
			return LineDone{action: "enable", err: fmt.Errorf("keychain.Set secret: %w", err)}
		}
		if err := keychain.Set(line.TokenKey, token); err != nil {
			return LineDone{action: "enable", err: fmt.Errorf("keychain.Set token: %w", err)}
		}
		cfg, err := session.Load()
		if err != nil {
			return LineDone{action: "enable", err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.LineEnabled = true
		cfg.LineUsername = name
		if err := session.Save(cfg); err != nil {
			return LineDone{action: "enable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return LineDone{action: "enable"}
	}
}

func disableLine() tea.Cmd {
	return func() tea.Msg {
		cfg, err := session.Load()
		if err != nil {
			return LineDone{action: "disable", err: fmt.Errorf("session.Load: %w", err)}
		}
		if !cfg.LineEnabled && keychain.Get(line.SecretKey) == "" && keychain.Get(line.TokenKey) == "" {
			return LineDone{action: "disable"}
		}
		if err := keychain.Delete(line.SecretKey); err != nil {
			slog.Warn("keychain.Delete line secret",
				slog.String("error", err.Error()))
		}
		if err := keychain.Delete(line.TokenKey); err != nil {
			slog.Warn("keychain.Delete line token",
				slog.String("error", err.Error()))
		}
		cfg.LineEnabled = false
		if err := session.Save(cfg); err != nil {
			return LineDone{action: "disable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return LineDone{action: "disable"}
	}
}

func verifyLine(secret, token string) (string, error) {
	client, err := go_bot_line.New(secret, token, "0")
	if err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-bot/line New: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		return "", fmt.Errorf("github.com/pardnchiu/go-bot/line Start: %w", err)
	}
	name := client.Status().DisplayName
	_ = client.Close()
	return name, nil
}

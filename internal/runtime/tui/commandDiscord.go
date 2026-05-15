package tui

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

type DiscordAction struct {
	action string
}

type DiscordTokenSubmit struct {
	token string
}

type DiscordDone struct {
	action string
	err    error
}

func (t TUI) commandDiscord(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable":
			action := parts[1]
			return t, func() tea.Msg { return DiscordAction{action: action} }, true
		}
	}

	enabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		enabled = cfg.DiscordEnabled && keychain.Get(discord.Key) != ""
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Discord",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return DiscordAction{action: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openDiscordTokenPrompt() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Discord Bot Token",
		subtitle: "from Developer Portal · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return DiscordTokenSubmit{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func enableDiscord(token string) tea.Cmd {
	return func() tea.Msg {
		if token == "" {
			return DiscordDone{action: "enable", err: fmt.Errorf("token is required")}
		}
		if err := verifyDiscord(token); err != nil {
			return DiscordDone{action: "enable", err: err}
		}
		if err := keychain.Set(discord.Key, token); err != nil {
			return DiscordDone{action: "enable", err: fmt.Errorf("keychain.Set: %w", err)}
		}

		cfg, err := session.Load()
		if err != nil {
			return DiscordDone{action: "enable", err: fmt.Errorf("session.Load: %w", err)}
		}

		cfg.DiscordEnabled = true
		if err := session.Save(cfg); err != nil {
			return DiscordDone{action: "enable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return DiscordDone{action: "enable"}
	}
}

func disableDiscord() tea.Cmd {
	return func() tea.Msg {
		cfg, err := session.Load()
		if err != nil {
			return DiscordDone{action: "disable", err: fmt.Errorf("session.Load: %w", err)}
		}
		if !cfg.DiscordEnabled && keychain.Get(discord.Key) == "" {
			return DiscordDone{action: "disable"}
		}
		_ = keychain.Delete(discord.Key)
		cfg.DiscordEnabled = false
		if err := session.Save(cfg); err != nil {
			return DiscordDone{action: "disable", err: fmt.Errorf("session.Save: %w", err)}
		}
		return DiscordDone{action: "disable"}
	}
}

func verifyDiscord(token string) error {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return fmt.Errorf("github.com/bwmarrin/discordgo New: %w", err)
	}
	if err := s.Open(); err != nil {
		return fmt.Errorf("open gateway: %w", err)
	}
	return s.Close()
}

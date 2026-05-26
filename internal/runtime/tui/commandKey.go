package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
)

type KeySelect struct {
	key string
}

type KeySubmit struct {
	key   string
	value string
}

func (t TUI) commandKey(parts []string) (TUI, tea.Cmd, bool) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n"), true
	}
	if len(cfg.Keys) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no keys recorded · run /model add or store_secret first") + "\n"), true
	}

	if len(parts) > 1 {
		target := strings.TrimSpace(parts[1])
		if !slices.Contains(cfg.Keys, target) {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] key not recorded: %q", target)) + "\n"), true
		}
		next, cmd := t.openKeyValuePrompt(target)
		return next, cmd, true
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Key · update keychain value",
		options: cfg.Keys,
		values:  cfg.Keys,
		onConfirm: func(chosen string) any {
			return KeySelect{key: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openKeyValuePrompt(key string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    fmt.Sprintf("Key · %s", key),
		subtitle: "Enter new value · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return KeySubmit{key: key, value: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

func (t TUI) commandImage2(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable":
			if !openaicodex.HasToken() {
				next, cmd := t.startImage2CodexOAuth()
				return next, cmd, true
			}
			return t, setImage2("enable"), true
		case "disable":
			return t, setImage2("disable"), true
		}
	}

	enabled := false
	if cfg, err := config.Load(); err == nil && cfg != nil {
		enabled = cfg.EnableImage2
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Image2",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return Image2Action{action: chosen}
		},
	}
	return t, nil, true
}

type Image2Action struct {
	action string
}

type Image2Done struct {
	action string
	err    error
}

func setImage2(action string) tea.Cmd {
	return func() tea.Msg {
		if action == "enable" && !openaicodex.HasToken() {
			return Image2Done{action: action, err: fmt.Errorf("codex token is required; run /model global add and authenticate Codex first")}
		}
		cfg, err := config.Load()
		if err != nil {
			return Image2Done{action: action, err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.EnableImage2 = action == "enable"
		if err := config.Save(cfg); err != nil {
			return Image2Done{action: action, err: fmt.Errorf("session.Save: %w", err)}
		}
		return Image2Done{action: action}
	}
}

func (t TUI) startImage2CodexOAuth() (TUI, tea.Cmd) {
	t.enableImage2AfterOAuth = true
	t.modelAdd = &modelAddItem{provider: "codex"}
	return t.startOAuthPopup()
}

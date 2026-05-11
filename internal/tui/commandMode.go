package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ModeSelect struct {
	mode TUIMode
}

func (t TUI) commandMode(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "cli":
			next, cmd := t.runModeSelect(cliMode)
			return next, cmd, true
		case "web":
			next, cmd := t.runModeSelect(webMode)
			return next, cmd, true
		}
	}

	cursor := 0
	if t.mode == webMode {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Switch mode",
		options: []string{"cli", "web"},
		values:  []string{"cli", "web"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			switch chosen {
			case "cli":
				return ModeSelect{mode: cliMode}
			case "web":
				return ModeSelect{mode: webMode}
			}
			return nil
		},
	}
	return t, nil, true
}

func (t TUI) runModeSelect(mode TUIMode) (TUI, tea.Cmd) {
	if t.mode == mode {
		return t, nil
	}

	switch mode {
	case cliMode:
		t.mode = cliMode
		return t, nil
	case webMode:
		next, cmd, _ := t.webMode()
		return next, cmd
	}
	return t, nil
}

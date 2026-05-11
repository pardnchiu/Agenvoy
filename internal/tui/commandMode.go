package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ModeSelect struct {
	mode TUIMode
}

func (t TUI) commandMode() (TUI, tea.Cmd, bool) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Switch mode",
		options: []string{"cli", "log", "web"},
		values:  []string{"cli", "log", "web"},
		cursor:  int(t.mode),
		onConfirm: func(chosen string) any {
			switch chosen {
			case "cli":
				return ModeSelect{mode: cliMode}
			case "log":
				return ModeSelect{mode: logMode}
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

	if t.mode == logMode && t.logCancel != nil {
		t.logCancel()
		t.logCancel = nil
	}

	switch mode {
	case cliMode:
		t.mode = cliMode
		return t, nil
	case logMode:
		return t.logMode(true)
	case webMode:
		next, cmd, _ := t.webMode()
		return next, cmd
	}
	return t, nil
}

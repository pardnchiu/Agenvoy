package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type ModelScopeSelect struct {
	scope string
}

type ModelAction struct {
	action string
}

func (t TUI) commandModel(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "global":
			next, cmd := t.openModelGlobalPopup()
			return next, cmd, true
		case "session":
			return t.commandSessionModel()
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Model",
		options: []string{"global", "session"},
		values:  []string{"global", "session"},
		onConfirm: func(chosen string) any {
			return ModelScopeSelect{scope: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openModelGlobalPopup() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Model · global",
		options: []string{"add", "remove"},
		values:  []string{"add", "remove"},
		onConfirm: func(chosen string) any {
			return ModelAction{action: chosen}
		},
	}
	return t, nil
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type McpAction struct {
	action string
}

func (t TUI) commandMcp(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "add":
			return t.commandMcpAdd()
		case "remove", "rm":
			return t.commandMcpRemove()
		case "install":
			return t.commandMcpInstall()
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "MCP",
		options: []string{"add", "remove", "install  external agent config"},
		values:  []string{"add", "remove", "install"},
		onConfirm: func(chosen string) any {
			return McpAction{action: chosen}
		},
	}
	return t, nil, true
}

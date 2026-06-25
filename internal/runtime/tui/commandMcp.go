package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

type McpAction struct {
	action string
}

type McpReconnectDone struct {
	err error
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
		case "reconnect":
			return t.commandMcpReconnect()
		}
	}

	t.popup = &Popup{
		kind:        popupSingleSelect,
		title:       "MCP",
		styledLines: mcpStatusLines(),
		options:     []string{"add", "remove", "reconnect", "install  external agent config"},
		values:      []string{"add", "remove", "reconnect", "install"},
		onConfirm: func(chosen string) any {
			return McpAction{action: chosen}
		},
	}
	return t, nil, true
}

func mcpStatusLines() []string {
	m := mcp.Manager()
	if m == nil {
		return nil
	}
	list := m.Status("")
	if len(list) == 0 {
		return nil
	}
	maxName := 0
	for _, s := range list {
		if len(s.Name) > maxName {
			maxName = len(s.Name)
		}
	}
	lines := make([]string, len(list))
	for i, s := range list {
		prefix := fmt.Sprintf("  %-*s  %-5s  ", maxName, s.Name, s.Transport)
		if s.Connected {
			lines[i] = hintStyle.Render(prefix) + okayStyle.Render("● connected")
		} else {
			lines[i] = hintStyle.Render(prefix) + errorStyle.Render("○ disconnected")
		}
	}
	return lines
}

func (t TUI) commandMcpReconnect() (TUI, tea.Cmd, bool) {
	return t, func() tea.Msg {
		m := mcp.Manager()
		if m == nil {
			return McpReconnectDone{err: fmt.Errorf("no MCP manager")}
		}
		err := m.Reconnect(context.Background(), "")
		return McpReconnectDone{err: err}
	}, true
}

package tui

import (
	"fmt"
	"maps"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

type McpRemove struct {
	server string
}

func (t TUI) commandMcpRemove() (TUI, tea.Cmd, bool) {
	cfg, err := mcp.Load()
	if err != nil || len(cfg.Servers) == 0 {
		return t, tea.Println(hintStyle.Render("no mcp servers configured") + "\n"), true
	}

	names := slices.Sorted(maps.Keys(cfg.Servers))
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Remove mcp server",
		options: names,
		values:  names,
		onConfirm: func(chosen string) any {
			return McpRemove{server: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runMcpRemove(target McpRemove) (TUI, tea.Cmd) {
	cfg, err := mcp.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp.Load: %v", err)) + "\n")
	}
	if _, ok := cfg.Servers[target.server]; !ok {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp server %q not found", target.server)) + "\n")
	}
	delete(cfg.Servers, target.server)
	if err := mcp.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp.Save: %v", err)) + "\n")
	}
	next, cmd, _ := t.commandMcpReconnect()
	return next, tea.Batch(tea.Println(hintStyle.Render(fmt.Sprintf("⎯ removed: %s", target.server))), cmd)
}

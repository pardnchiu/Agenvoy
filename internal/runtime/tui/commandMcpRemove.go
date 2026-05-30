package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionBot "github.com/pardnchiu/agenvoy/internal/session/bot"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

type McpRemove struct {
	path   string
	scope  string
	server string
}

type mcpEntry struct {
	path   string
	scope  string
	server string
	cfg    mcp.ServerConfig
}

func listMcpEntries() []mcpEntry {
	var entries []mcpEntry

	if global, err := mcp.Load(filesystem.McpPath); err == nil {
		for _, name := range slices.Sorted(maps.Keys(global.Servers)) {
			entries = append(entries, mcpEntry{
				path:   filesystem.McpPath,
				scope:  "global",
				server: name,
				cfg:    global.Servers[name],
			})
		}
	}

	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return entries
	}
	for _, d := range dirs {
		sid := d.Name
		if !strings.HasPrefix(sid, "cli-") && !strings.HasPrefix(sid, "http-") {
			continue
		}
		cfg, err := mcp.Load(filesystem.McpSessionPath(sid))
		if err != nil {
			continue
		}
		name, _ := sessionBot.Get(sid)
		label := sid
		if name != "" && name != sid {
			label = fmt.Sprintf("%s (%s)", sid, name)
		}
		for _, server := range slices.Sorted(maps.Keys(cfg.Servers)) {
			entries = append(entries, mcpEntry{
				path:   filesystem.McpSessionPath(sid),
				scope:  label,
				server: server,
				cfg:    cfg.Servers[server],
			})
		}
	}
	return entries
}

func (t TUI) commandMcpRemove() (TUI, tea.Cmd, bool) {
	entries := listMcpEntries()
	if len(entries) == 0 {
		return t, tea.Println(hintStyle.Render("no mcp servers configured") + "\n"), true
	}

	options := make([]string, len(entries))
	values := make([]string, len(entries))
	lookup := make(map[string]McpRemove, len(entries))
	for i, e := range entries {
		key := fmt.Sprintf("%d|%s|%s", i, e.scope, e.server)
		label := fmt.Sprintf("%s  %s", e.server, hintStyle.Render("("+e.scope+")"))
		options[i] = label
		values[i] = key
		lookup[key] = McpRemove{
			path:   e.path,
			scope:  e.scope,
			server: e.server,
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Remove mcp server",
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return lookup[chosen]
		},
	}
	return t, nil, true
}

func (t TUI) runMcpRemove(target McpRemove) (TUI, tea.Cmd) {
	cfg, err := mcp.Load(target.path)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp.Load: %v", err)) + "\n")
	}
	if _, ok := cfg.Servers[target.server]; !ok {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp server %q not found in %s", target.server, target.scope)) + "\n")
	}
	delete(cfg.Servers, target.server)
	if err := mcp.Save(target.path, cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp.Save: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ removed: %s (%s) · restart daemon to apply", target.server, target.scope)) + "\n")
}

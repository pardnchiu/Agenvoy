package tui

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	agentsexecallowtool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
)

func allowAllEntry(server string) string {
	return mcpToolPrefix(server) + "*"
}

func mcpToolPrefix(server string) string {
	return "mcp__" + server + "__"
}

type McpPermissionResult struct {
	server string
	tools  []mcp.Tool
	err    error
	back   *Popup
}

type McpPermissionPick struct {
	server  string
	entries []string
}

func (t TUI) openMcpPermission(name string) (TUI, tea.Cmd) {
	back := t.popupOrigin
	return t, func() tea.Msg {
		result, err := ipcClient.Load().MCP("tools", name)
		if err != nil {
			return McpPermissionResult{server: name, err: err, back: back}
		}
		return McpPermissionResult{server: name, tools: result.Tools, back: back}
	}
}

func (t TUI) runMcpPermissionResult(msg McpPermissionResult) (TUI, tea.Cmd) {
	if msg.err != nil {
		return t, notice(msgError(fmt.Sprintf("%s tools: %v", msg.server, msg.err)) + "\n")
	}
	if len(msg.tools) == 0 {
		slog.Debug("tui: server exposes no tools", slog.String("server", msg.server))
		return t, nil
	}

	nameSummary := make(map[string]string, len(msg.tools))
	for _, tool := range msg.tools {
		if trimmed := strings.TrimSpace(tool.Name); trimmed != "" {
			nameSummary[trimmed], _, _ = strings.Cut(strings.TrimSpace(tool.Description), "\n")
		}
	}
	names := slices.Sorted(maps.Keys(nameSummary))
	summaries := make([]string, len(names))
	for i, name := range names {
		summaries[i] = nameSummary[name]
	}

	granted := agentsexecallowtool.Load()
	all := allowAllEntry(msg.server)

	options := make([]string, 0, len(names)+1)
	values := make([]string, 0, len(names)+1)
	multi := make(map[int]bool, len(names)+1)

	options = append(options, "all  every tool of this server")
	values = append(values, all)
	if granted[all] {
		multi[0] = true
	}

	options = append(options, optionColumn(names, summaries)...)
	for _, name := range names {
		entry := mcpToolPrefix(msg.server) + name
		values = append(values, entry)
		if granted[entry] {
			multi[len(values)-1] = true
		}
	}

	if multi[0] {
		for i := 1; i <= len(names); i++ {
			multi[i] = true
		}
	} else if len(names) > 0 {
		multi[0] = true
		for i := 1; i <= len(names); i++ {
			if !multi[i] {
				multi[0] = false
				break
			}
		}
	}

	server := msg.server
	t.popup = &Popup{
		kind:     popupMultiSelect,
		title:    "MCP  " + server + "  tools",
		subtitle: "selected tools skip the confirmation prompt in every work directory",
		options:  options,
		values:   values,
		multi:    multi,
		onToggle: syncAllRow,
		back:     msg.back,
		onConfirm: func(chosen string) any {
			pick := McpPermissionPick{server: server}
			if chosen != "" {
				pick.entries = strings.Split(chosen, "\x1F")
			}
			return pick
		},
	}
	return t, nil
}

func syncAllRow(p *Popup, index int) {
	if len(p.options) < 2 {
		return
	}

	if index == 0 {
		for i := 1; i < len(p.options); i++ {
			p.multi[i] = p.multi[0]
		}
		return
	}

	for i := 1; i < len(p.options); i++ {
		if !p.multi[i] {
			p.multi[0] = false
			return
		}
	}
	p.multi[0] = true
}

func (t TUI) runMcpPermissionPick(msg McpPermissionPick) (TUI, tea.Cmd) {
	entries := msg.entries
	if slices.Contains(entries, allowAllEntry(msg.server)) {
		entries = []string{allowAllEntry(msg.server)}
	}

	if err := agentsexecallowtool.ToggleMCP(mcpToolPrefix(msg.server), entries); err != nil {
		return t, notice(msgError(fmt.Sprintf("%s permission: %v", msg.server, err)) + "\n")
	}

	slog.Debug("mcp permission updated", slog.String("server", msg.server), slog.Int("allowed", len(entries)))
	return t, nil
}

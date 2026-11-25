package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
)

type McpMenuPick struct {
	value string
}

type McpServerAction struct {
	server string
	action string
}

type McpReconnectDone struct {
	server string
	err    error
}

func (t TUI) commandMcp(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		if parts[1] == "add" {
			return t.commandMcpAdd()
		}
		if _, ok := mcpServerStatus(parts[1]); ok {
			next, cmd := t.openMcpServerMenu(parts[1])
			return next, cmd, true
		}
	}

	next, cmd := t.openLoading("/mcp", mcpPopup)
	return next, cmd, true
}

func mcpPopup() *Popup {
	list := mcpStatusList()
	options := make([]string, 0, len(list)+2)
	values := make([]string, 0, len(list)+2)
	tails := make([]string, 0, len(list)+2)

	maxName, maxTransport := 0, 0
	for _, s := range list {
		maxName = max(maxName, lipgloss.Width(s.Name))
		maxTransport = max(maxTransport, lipgloss.Width(s.Transport))
	}
	for _, s := range list {
		options = append(options, padToWidth(s.Name, maxName+2)+padToWidth(s.Transport, maxTransport+1))
		values = append(values, "server:"+s.Name)
		tails = append(tails, mcpStateLabel(s))
	}
	if len(list) > 0 {
		options = append(options, "")
		values = append(values, "")
		tails = append(tails, "")
	}
	options = append(options, "add")
	values = append(values, "add")
	tails = append(tails, "")

	return &Popup{
		kind:       popupSingleSelect,
		title:      "/mcp",
		options:    options,
		optionTail: tails,
		values:     values,
		maxVisible: cmdSelectorMaxVisible,
		onConfirm: func(chosen string) any {
			return McpMenuPick{value: chosen}
		},
		onDelete: func(chosen string) any {
			if name, ok := strings.CutPrefix(chosen, "server:"); ok {
				return McpRemovePick{server: name}
			}
			return nil
		},
	}
}

func mcpStatusList() []mcp.ServerInfo {
	result, err := ipcClient.Load().MCP("status", "")
	if err != nil {
		return nil
	}
	return result.Servers
}

func mcpServerStatus(name string) (mcp.ServerInfo, bool) {
	for _, s := range mcpStatusList() {
		if s.Name == name {
			return s, true
		}
	}
	return mcp.ServerInfo{}, false
}

func mcpStateLabel(s mcp.ServerInfo) string {
	switch {
	case !s.Connected:
		return errorStyle.Render("[disconnected]")
	case s.Error != "":
		return warnStyle.Render("[tools unavailable]")
	default:
		return okayStyle.Render("[connected]")
	}
}

func (t TUI) openMcpServerMenu(name string) (TUI, tea.Cmd) {
	info, ok := mcpServerStatus(name)
	if !ok {
		return t, notice(msgError(fmt.Sprintf("mcp server %q not found", name)) + "\n")
	}

	subtitle := fmt.Sprintf("%s  %s", info.Transport, mcpStateLabel(info))
	if info.Error != "" {
		subtitle = fmt.Sprintf("%s\n%s", subtitle, errorStyle.Render(info.Error))
	}

	values := []string{"tools", "reconnect"}
	details := []string{"pick always-allowed tools", ""}

	cfg, err := mcp.Load()
	if err == nil && cfg.Servers[name].IsOAuth() {
		login, detail := "login", "browser oauth"
		if mcp.HasOAuth(name) {
			login, detail = "relogin", "browser oauth  a token is already stored"
		}
		values = append([]string{login, "client"}, values...)
		details = append([]string{detail, "set oauth client id / secret"}, details...)
	}
	options := optionColumn(values, details)

	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "/mcp " + name,
		subtitle: subtitle,
		options:  options,
		values:   values,
		onConfirm: func(chosen string) any {
			return McpServerAction{server: name, action: chosen}
		},
	}
	return t, nil
}

func (t TUI) runMcpMenuPick(value string) (TUI, tea.Cmd) {
	if value == "add" {
		next, cmd, _ := t.commandMcpAdd()
		return next, cmd
	}
	if name, ok := strings.CutPrefix(value, "server:"); ok {
		return t.openMcpServerMenu(name)
	}
	return t, nil
}

func (t TUI) runMcpServerAction(msg McpServerAction) (TUI, tea.Cmd) {
	switch msg.action {
	case "login", "relogin":
		return t.startMcpLogin(msg.server)
	case "client":
		return t.openMcpClientID(msg.server)
	case "reconnect":
		return t.reconnectMcpServer(msg.server)
	case "tools":
		return t.openMcpPermission(msg.server)
	}
	return t, nil
}

func (t TUI) reconnectMcpServer(name string) (TUI, tea.Cmd) {
	return t, func() tea.Msg {
		_, err := ipcClient.Load().MCP("reconnect", name)
		return McpReconnectDone{server: name, err: err}
	}
}

package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	toml "github.com/pelletier/go-toml/v2"
)

type mcpInstallClient struct {
	name       string
	configPath string
	format     string
	entry      map[string]any
}

type McpInstallPick struct {
	index int
}

type McpInstallDone struct {
	client string
	path   string
	err    error
}

var mcpInstallClients []mcpInstallClient

func init() {
	home, _ := os.UserHomeDir()
	mcpInstallClients = []mcpInstallClient{
		{
			name:       "Claude Code",
			configPath: filepath.Join(home, ".claude.json"),
			format:     "json",
			entry:      map[string]any{"command": "agen"},
		},
		{
			name:       "Codex",
			configPath: filepath.Join(home, ".codex", "config.toml"),
			format:     "toml",
			entry:      map[string]any{"command": "agen"},
		},
		{
			name:       "OpenCode",
			configPath: filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
			format:     "json-opencode",
			entry:      map[string]any{"type": "local", "command": []string{"agen"}},
		},
	}
}

func (t TUI) commandMcpInstall() (TUI, tea.Cmd, bool) {
	options := make([]string, len(mcpInstallClients))
	values := make([]string, len(mcpInstallClients))
	for i, c := range mcpInstallClients {
		options[i] = fmt.Sprintf("%s  (%s)", c.name, c.configPath)
		values[i] = fmt.Sprintf("%d", i)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Install Agenvoy MCP server to",
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			var idx int
			fmt.Sscanf(chosen, "%d", &idx)
			return McpInstallPick{index: idx}
		},
	}
	return t, nil, true
}

func runMcpInstall(c mcpInstallClient) tea.Msg {
	var err error
	switch c.format {
	case "json":
		err = mcpInstallJSON(c.configPath, "mcpServers", c.entry)
	case "json-opencode":
		err = mcpInstallJSON(c.configPath, "mcp", c.entry)
	case "toml":
		err = mcpInstallTOML(c.configPath, c.entry)
	default:
		err = fmt.Errorf("unsupported format: %s", c.format)
	}
	return McpInstallDone{client: c.name, path: c.configPath, err: err}
}

func mcpInstallJSON(path, serversKey string, entry map[string]any) error {
	doc := map[string]any{}
	if text, err := go_pkg_filesystem.ReadText(path); err == nil {
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}

	servers, _ := doc[serversKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers["agenvoy"]; exists {
		return fmt.Errorf("agenvoy already configured")
	}
	servers["agenvoy"] = entry
	doc[serversKey] = servers

	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return go_pkg_filesystem.WriteFile(path, string(out)+"\n", 0644)
}

func mcpInstallTOML(path string, entry map[string]any) error {
	doc := map[string]any{}
	if text, err := go_pkg_filesystem.ReadText(path); err == nil {
		if err := toml.Unmarshal([]byte(text), &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}

	servers, _ := doc["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers["agenvoy"]; exists {
		return fmt.Errorf("agenvoy already configured")
	}
	servers["agenvoy"] = entry
	doc["mcp_servers"] = servers

	out, err := toml.Marshal(doc)
	if err != nil {
		return err
	}
	return go_pkg_filesystem.WriteFile(path, string(out), 0644)
}

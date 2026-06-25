package tui

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

type mcpAddDraft struct {
	name           string
	transport      string
	command        string
	args           []string
	env            map[string]string
	url            string
	headers        map[string]string
	authHeaderName string
	scope          string
	sessionID      string
}

type McpAddName struct {
	name string
}

type McpAddTransport struct {
	transport string
}

type McpAddCommand struct {
	command string
}

type McpAddArgs struct {
	raw string
}

type McpAddEnv struct {
	raw string
}

type McpAddURL struct {
	url string
}

type McpAddAuthMethod struct {
	method string
}

type McpAddBearerToken struct {
	token string
}

type McpAddAPIKeyHeader struct {
	header string
}

type McpAddAPIKeyValue struct {
	value string
}

type McpAddBasicToken struct {
	token string
}

type McpAddHeaders struct {
	raw string
}

type McpAddScope struct {
	scope string
}

type McpAddSessionPick struct {
	id string
}

type McpAddSaved struct {
	name  string
	scope string
	err   error
}

func (t TUI) commandMcpAdd() (TUI, tea.Cmd, bool) {
	t.mcpAdd = &mcpAddDraft{}
	t.popup = &Popup{
		kind:  popupText,
		title: "MCP server name",
		onConfirm: func(value string) any {
			return McpAddName{name: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) openMcpAddTransport() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Transport",
		options: []string{"stdio            local command", "streamable-http  remote server"},
		values:  []string{"stdio", "streamable-http"},
		onConfirm: func(chosen string) any {
			return McpAddTransport{transport: chosen}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddCommand() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "Command (executable path or name)",
		onConfirm: func(value string) any {
			return McpAddCommand{command: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddArgs() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Args (comma-separated, blank to skip)",
		subtitle: "example: --port,8080,--config,./cfg.json",
		onConfirm: func(value string) any {
			return McpAddArgs{raw: value}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddEnv() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		multiline: true,
		title:     "Env (KEY=VALUE per line · ctrl+s submit · blank to skip)",
		subtitle:  "example:\nAPI_KEY=${MY_KEY}\nREGION=us-west-1",
		onConfirm: func(value string) any {
			return McpAddEnv{raw: value}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddURL() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupText,
		title: "URL",
		onConfirm: func(value string) any {
			return McpAddURL{url: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddHeaders() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		multiline: true,
		title:     "Extra headers (KEY=VALUE per line · ctrl+s submit · blank to skip)",
		subtitle:  "example:\nX-Trace=1\nX-Client=agenvoy",
		onConfirm: func(value string) any {
			return McpAddHeaders{raw: value}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddAuthMethod() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:  popupSingleSelect,
		title: "Authentication",
		options: []string{
			"none    no auth",
			"bearer  Authorization: Bearer token",
			"api key custom header token",
			"basic   Authorization: Basic token",
		},
		values: []string{"none", "bearer", "apikey", "basic"},
		onConfirm: func(chosen string) any {
			return McpAddAuthMethod{method: chosen}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddBearerToken() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Bearer token",
		subtitle: "raw token or ${TOKEN}; Bearer is added automatically",
		onConfirm: func(value string) any {
			return McpAddBearerToken{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddAPIKeyHeader() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "API key header name",
		subtitle: "blank uses X-API-Key",
		onConfirm: func(value string) any {
			return McpAddAPIKeyHeader{header: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddAPIKeyValue(header string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    fmt.Sprintf("%s value", header),
		subtitle: "raw key or ${TOKEN}",
		onConfirm: func(value string) any {
			return McpAddAPIKeyValue{value: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddBasicToken() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Basic auth token",
		subtitle: "base64 user:pass or ${BASIC_TOKEN}; Basic is added automatically",
		onConfirm: func(value string) any {
			return McpAddBasicToken{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddScope() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Scope",
		options: []string{"global   all sessions", "session  pick one"},
		values:  []string{"global", "session"},
		onConfirm: func(chosen string) any {
			return McpAddScope{scope: chosen}
		},
	}
	return t, nil
}

func (t TUI) openMcpAddSessionPick() (TUI, tea.Cmd) {
	sessions := availableSessions()
	if len(sessions) == 0 {
		err := fmt.Errorf("no sessions available")
		t.mcpAdd = nil
		return t, func() tea.Msg { return McpAddSaved{err: err} }
	}
	options := make([]string, len(sessions))
	values := make([]string, len(sessions))
	for i, s := range sessions {
		label := s.id
		if s.name != "" && s.name != s.id {
			label = fmt.Sprintf("%s (%s)", s.id, s.name)
		}
		options[i] = label
		values[i] = s.id
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Session",
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return McpAddSessionPick{id: chosen}
		},
	}
	return t, nil
}

type sessionRef struct {
	id   string
	name string
}

func availableSessions() []sessionRef {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}
	out := make([]sessionRef, 0, len(dirs))
	for _, d := range dirs {
		sid := d.Name
		if !strings.HasPrefix(sid, "cli-") && !strings.HasPrefix(sid, "http-") {
			continue
		}
		name, _ := configBot.Get(sid)
		out = append(out, sessionRef{id: sid, name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func parseKV(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeMcpHeaders(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(extra))
	maps.Copy(out, base)
	maps.Copy(out, extra)
	return out
}

func bearerAuthorizationHeader(token string) map[string]string {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if strings.Contains(token, " ") {
		return map[string]string{"Authorization": token}
	}
	return map[string]string{"Authorization": "Bearer " + token}
}

func apiKeyHeader(header, value string) map[string]string {
	header = strings.TrimSpace(header)
	value = strings.TrimSpace(value)
	if header == "" {
		header = "X-API-Key"
	}
	if value == "" {
		return nil
	}
	return map[string]string{header: value}
}

func basicAuthorizationHeader(token string) map[string]string {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if strings.Contains(token, " ") {
		return map[string]string{"Authorization": token}
	}
	return map[string]string{"Authorization": "Basic " + token}
}

func parseArgsCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (t TUI) finalizeMcpAdd() (TUI, tea.Cmd) {
	d := t.mcpAdd
	t.mcpAdd = nil
	if d == nil {
		return t, nil
	}

	cfg := mcp.ServerConfig{}
	switch d.transport {
	case "stdio":
		cfg.Command = d.command
		cfg.Args = d.args
		cfg.Env = d.env
	case "streamable-http":
		cfg.URL = d.url
		cfg.Headers = d.headers
	}

	var path, scopeLabel string
	switch d.scope {
	case "global":
		path = filesystem.McpPath
		scopeLabel = "global"
	case "session":
		path = filesystem.McpSessionPath(d.sessionID)
		scopeLabel = d.sessionID
	default:
		return t, func() tea.Msg { return McpAddSaved{name: d.name, err: fmt.Errorf("invalid scope")} }
	}

	existing, err := mcp.Load(path)
	if err != nil {
		return t, func() tea.Msg {
			return McpAddSaved{name: d.name, scope: scopeLabel, err: fmt.Errorf("mcp.Load: %w", err)}
		}
	}
	if existing.Servers == nil {
		existing.Servers = map[string]mcp.ServerConfig{}
	}
	existing.Servers[d.name] = cfg
	if err := mcp.Save(path, existing); err != nil {
		return t, func() tea.Msg {
			return McpAddSaved{name: d.name, scope: scopeLabel, err: fmt.Errorf("mcp.Save: %w", err)}
		}
	}
	return t, func() tea.Msg { return McpAddSaved{name: d.name, scope: scopeLabel} }
}

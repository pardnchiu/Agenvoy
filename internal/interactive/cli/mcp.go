package cli

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/toolAdapter/mcp"
)

func MCP(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if sub == "" {
		sub = Pick("Select MCP action", []string{"list", "add", "remove"})
	}
	switch sub {
	case "list":
		runMCPList()
	case "add":
		runMCPAdd()
	case "remove", "rm":
		runMCPRemove()
	default:
		fmt.Fprintf(os.Stderr, "Usage: agen mcp [list|add|remove]\n")
		os.Exit(1)
	}
}

type sessionInfo struct {
	id   string
	name string
}

func listSessions() []sessionInfo {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}
	out := make([]sessionInfo, 0, len(dirs))
	for _, d := range dirs {
		sid := d.Name
		if !strings.HasPrefix(sid, "cli-") && !strings.HasPrefix(sid, "http-") {
			continue
		}
		name, _ := session.GetBot(sid)
		out = append(out, sessionInfo{id: sid, name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func runMCPList() {
	type entry struct {
		scope  string
		server string
		cfg    mcp.ServerConfig
	}

	var entries []entry

	if global, err := mcp.Load(filesystem.McpPath); err == nil {
		for _, name := range slices.Sorted(maps.Keys(global.Servers)) {
			entries = append(entries, entry{scope: "global", server: name, cfg: global.Servers[name]})
		}
	}

	for _, s := range listSessions() {
		cfg, err := mcp.Load(filesystem.McpSessionPath(s.id))
		if err != nil {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(cfg.Servers)) {
			label := s.id
			if s.name != "" && s.name != s.id {
				label = fmt.Sprintf("%s (%s)", s.id, s.name)
			}
			entries = append(entries, entry{scope: label, server: name, cfg: cfg.Servers[name]})
		}
	}

	if len(entries) == 0 {
		fmt.Println("No MCP servers configured.")
		return
	}

	for _, e := range entries {
		transport := "stdio"
		target := e.cfg.Command
		if e.cfg.IsHTTP() {
			transport = "http"
			target = e.cfg.URL
		}
		fmt.Printf("• %s  scope=%s  transport=%s  target=%s\n", e.server, e.scope, transport, target)
	}
}

func runMCPAdd() {
	name := promptText("Server name", "")
	if name == "" {
		fmt.Fprintln(os.Stderr, "name is required")
		os.Exit(1)
	}

	transportSel := promptui.Select{
		Label:        "Type",
		Items:        []string{"Local (stdio)", "Remote (HTTP)"},
		HideSelected: true,
	}
	tIdx, _, err := transportSel.Run()
	if err != nil {
		os.Exit(1)
	}

	cfg := mcp.ServerConfig{}
	switch tIdx {
	case 0:
		cfg.Command = promptText("Command", "")
		if cfg.Command == "" {
			fmt.Fprintln(os.Stderr, "command is required")
			os.Exit(1)
		}
		argsRaw := promptText("Args (comma-separated)", "")
		cfg.Args = parseCSV(argsRaw)
		cfg.Env = promptKV("Environment variables (KEY=VALUE, blank to finish)")
	case 1:
		cfg.URL = promptText("URL", "")
		if cfg.URL == "" {
			fmt.Fprintln(os.Stderr, "url is required")
			os.Exit(1)
		}
		cfg.Headers = promptKV("Headers (KEY=VALUE, blank to finish)")
	}

	scopeSel := promptui.Select{
		Label:        "Scope",
		Items:        []string{"Global (all sessions)", "Session (select session)"},
		HideSelected: true,
	}
	sIdx, _, err := scopeSel.Run()
	if err != nil {
		os.Exit(1)
	}

	var path string
	var scopeLabel string
	switch sIdx {
	case 0:
		path = filesystem.McpPath
		scopeLabel = "global"
	case 1:
		sessions := listSessions()
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "no sessions available")
			os.Exit(1)
		}
		labels := make([]string, len(sessions))
		for i, s := range sessions {
			if s.name != "" && s.name != s.id {
				labels[i] = fmt.Sprintf("%s (%s)", s.id, s.name)
			} else {
				labels[i] = s.id
			}
		}
		sessionSel := promptui.Select{
			Label:        "Session",
			Items:        labels,
			HideSelected: true,
		}
		idx, _, err := sessionSel.Run()
		if err != nil {
			os.Exit(1)
		}
		path = filesystem.McpSessionPath(sessions[idx].id)
		scopeLabel = sessions[idx].id
	}

	existing, err := mcp.Load(path)
	if err != nil {
		slog.Error("mcp.Load", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if existing.Servers == nil {
		existing.Servers = map[string]mcp.ServerConfig{}
	}
	existing.Servers[name] = cfg
	if err := mcp.Save(path, existing); err != nil {
		slog.Error("mcp.Save", slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("[*] Added %q to %s mcp.json\n", name, scopeLabel)
}

func runMCPRemove() {
	type item struct {
		path  string
		scope string
		name  string
	}
	var items []item

	if cfg, err := mcp.Load(filesystem.McpPath); err == nil {
		for _, name := range slices.Sorted(maps.Keys(cfg.Servers)) {
			items = append(items, item{path: filesystem.McpPath, scope: "global", name: name})
		}
	}
	for _, s := range listSessions() {
		cfg, err := mcp.Load(filesystem.McpSessionPath(s.id))
		if err != nil {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(cfg.Servers)) {
			label := s.id
			if s.name != "" && s.name != s.id {
				label = fmt.Sprintf("%s (%s)", s.id, s.name)
			}
			items = append(items, item{path: filesystem.McpSessionPath(s.id), scope: label, name: name})
		}
	}

	if len(items) == 0 {
		fmt.Println("No MCP servers configured.")
		return
	}

	labels := make([]string, len(items)+1)
	for i, it := range items {
		labels[i] = fmt.Sprintf("%s (%s)", it.name, it.scope)
	}
	labels[len(items)] = "exit"

	sel := promptui.Select{
		Label:        "Server",
		Items:        labels,
		HideSelected: true,
	}
	idx, _, err := sel.Run()
	if err != nil {
		os.Exit(1)
	}
	if idx == len(items) {
		return
	}
	target := items[idx]

	if !promptYesNo(fmt.Sprintf("Confirm remove %q", target.name), false) {
		return
	}

	cfg, err := mcp.Load(target.path)
	if err != nil {
		slog.Error("mcp.Load", slog.String("error", err.Error()))
		os.Exit(1)
	}
	delete(cfg.Servers, target.name)
	if err := mcp.Save(target.path, cfg); err != nil {
		slog.Error("mcp.Save", slog.String("error", err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[*] Removed %q\n", target.name)
}

func promptText(label, def string) string {
	p := promptui.Prompt{Label: label, Default: def}
	v, err := p.Run()
	if err != nil {
		os.Exit(1)
	}
	return strings.TrimSpace(v)
}

func promptYesNo(label string, def bool) bool {
	defStr := "y"
	if !def {
		defStr = "n"
	}
	p := promptui.Prompt{
		Label:   label + " (y/n)",
		Default: defStr,
		Validate: func(s string) error {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "y", "yes", "n", "no", "":
				return nil
			}
			return fmt.Errorf("y or n")
		},
	}
	v, err := p.Run()
	if err != nil {
		os.Exit(1)
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return def
	}
	return v == "y" || v == "yes"
}

func parseCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func promptKV(label string) map[string]string {
	out := map[string]string{}
	for {
		p := promptui.Prompt{
			Label:   label,
			Default: "",
		}
		v, err := p.Run()
		if err != nil {
			os.Exit(1)
		}
		v = strings.TrimSpace(v)
		if v == "" {
			break
		}
		eq := strings.IndexByte(v, '=')
		if eq <= 0 {
			fmt.Fprintln(os.Stderr, "expected KEY=VALUE")
			continue
		}
		key := strings.TrimSpace(v[:eq])
		val := strings.TrimSpace(v[eq+1:])
		if key == "" {
			fmt.Fprintln(os.Stderr, "empty key")
			continue
		}
		out[key] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

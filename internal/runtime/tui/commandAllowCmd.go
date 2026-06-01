package tui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type AllowCmdSubmit struct {
	name string
}

func (t TUI) commandAllowCmd(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) >= 2 {
		name := strings.TrimSpace(strings.Join(parts[1:], " "))
		next, cmd := t.runAllowCmdAppend(name)
		return next, cmd, true
	}
	t.popup = &Popup{
		kind:  popupText,
		title: "Command to allow (appended to config.json white_list)",
		onConfirm: func(value string) any {
			return AllowCmdSubmit{name: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) runAllowCmdAppend(name string) (TUI, tea.Cmd) {
	if name == "" {
		return t, tea.Println(errorStyle.Render("[!] command name required") + "\n")
	}
	if slices.Contains(filesystem.WhiteList, name) {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s already allowed", name)) + "\n")
	}

	dic := map[string]json.RawMessage{}
	if go_pkg_filesystem_reader.Exists(filesystem.ConfigPath) {
		loaded, err := go_pkg_filesystem.ReadJSON[map[string]json.RawMessage](filesystem.ConfigPath)
		if err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] read config: %v", err)) + "\n")
		}
		dic = loaded
	}
	var current []string
	if data, ok := dic["white_list"]; ok && len(data) > 0 {
		if err := json.Unmarshal(data, &current); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] parse white_list: %v", err)) + "\n")
		}
	}
	if slices.Contains(current, name) {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s already in config.white_list", name)) + "\n")
	}
	current = append(current, name)

	raw, err := json.Marshal(current)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] marshal white_list: %v", err)) + "\n")
	}
	dic["white_list"] = raw
	if err := go_pkg_filesystem.CheckDir(filesystem.AgenvoyDir, true); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mkdir agenvoy: %v", err)) + "\n")
	}
	if err := go_pkg_filesystem.WriteJSON(filesystem.ConfigPath, dic, false); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] write config: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ added to config.white_list: %s · restart daemon (agen stop) to apply", name)) + "\n")
}

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type ModelRemove struct {
	name string
}

func (t TUI) commandModelRemove() (TUI, tea.Cmd, bool) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n"), true
	}
	if len(cfg.Models) == 0 {
		return t, tea.Println(hintStyle.Render("no models configured") + "\n"), true
	}

	options := make([]string, len(cfg.Models))
	values := make([]string, len(cfg.Models))
	for i, m := range cfg.Models {
		label := m.Name
		if m.Description != "" {
			label = fmt.Sprintf("%s  %s", m.Name, hintStyle.Render(m.Description))
		}
		if cfg.DispatcherModel != "" && m.Name == cfg.DispatcherModel {
			label += "  " + warnStyle.Render("[dispatcher]")
		}
		options[i] = label
		values[i] = m.Name
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Remove model",
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return ModelRemove{name: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runModelRemove(name string) (TUI, tea.Cmd) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}

	if len(cfg.Models) == 0 {
		return t, tea.Println(hintStyle.Render("no models configured") + "\n")
	}

	idx := -1
	for i, m := range cfg.Models {
		if m.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] model %q not found", name)) + "\n")
	}

	cfg.Models = append(cfg.Models[:idx], cfg.Models[idx+1:]...)
	clearedDispatcher := false
	if cfg.DispatcherModel == name {
		cfg.DispatcherModel = ""
		clearedDispatcher = true
	}

	if err := session.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}

	lines := []string{hintStyle.Render(fmt.Sprintf("⎯ removed: %s", name))}
	if clearedDispatcher {
		lines = append(lines, warnStyle.Render("dispatcher cleared · run /model or set a new dispatcher"))
	}
	if len(cfg.Models) == 0 {
		lines = append(lines, warnStyle.Render("⎯ no model configured · /model global add"))
	}
	return t, tea.Println(strings.Join(lines, "\n\n") + "\n")
}

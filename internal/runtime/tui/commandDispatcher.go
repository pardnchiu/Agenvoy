package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session/config"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

type DispatcherSelect struct {
	name string
}

func (t TUI) commandDispatcher() (TUI, tea.Cmd, bool) {
	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n"), true
	}
	if len(cfg.Models) == 0 {
		return t, tea.Println(hintStyle.Render("no models configured · use /model") + "\n"), true
	}

	options := make([]string, len(cfg.Models))
	values := make([]string, len(cfg.Models))
	cursor := 0
	for i, m := range cfg.Models {
		label := m.Name
		if m.Description != "" {
			label = fmt.Sprintf("%s  %s", m.Name, hintStyle.Render(m.Description))
		}
		if cfg.DispatcherModel != "" && m.Name == cfg.DispatcherModel {
			label += "  " + systemStyle.Render("[current]")
			cursor = i
		}
		options[i] = label
		values[i] = m.Name
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Select dispatcher model",
		options: options,
		values:  values,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return DispatcherSelect{name: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) cycleDispatcher(forward bool) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, nil
	}

	cfg, err := config.Load()
	if err != nil || len(cfg.Models) == 0 {
		return t, nil
	}

	candidates := make([]string, 0, len(cfg.Models)+1)
	candidates = append(candidates, configBot.DefaultModel)
	for _, m := range cfg.Models {
		candidates = append(candidates, m.Name)
	}

	currentModel, _ := configBot.GetModel(sid)
	current := 0
	for i, c := range candidates {
		if c == currentModel {
			current = i
			break
		}
	}

	n := len(candidates)
	var next int
	if forward {
		next = (current + 1) % n
	} else {
		next = (current - 1 + n) % n
	}

	picked := candidates[next]
	configBot.SetModel(sid, picked, "")
	return t, nil
}

func (t TUI) runDispatcherSelect(name string) (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}
	if cfg.DispatcherModel == name {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ dispatcher unchanged: %s", name)) + "\n")
	}

	cfg.DispatcherModel = name
	if err := config.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ dispatcher: %s", name)) + "\n")
}

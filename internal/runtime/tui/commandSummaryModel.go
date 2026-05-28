package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type SummaryModelSelect struct {
	name string
}

func (t TUI) commandSummaryModel() (TUI, tea.Cmd, bool) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n"), true
	}
	if len(cfg.Models) == 0 {
		return t, tea.Println(hintStyle.Render("no models configured · use /model") + "\n"), true
	}

	options := make([]string, 0, len(cfg.Models)+1)
	values := make([]string, 0, len(cfg.Models)+1)
	cursor := 0

	options = append(options, hintStyle.Render("(use dispatcher)"))
	values = append(values, "")
	if cfg.SummaryModel == "" {
		options[0] += "  " + hintStyle.Render("[current]")
	}

	for i, m := range cfg.Models {
		label := m.Name
		if m.Description != "" {
			label = fmt.Sprintf("%s  %s", m.Name, hintStyle.Render(m.Description))
		}
		if cfg.SummaryModel != "" && m.Name == cfg.SummaryModel {
			label += "  " + hintStyle.Render("[current]")
			cursor = i + 1
		}
		options = append(options, label)
		values = append(values, m.Name)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Select summary model",
		options: options,
		values:  values,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return SummaryModelSelect{name: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runSummaryModelSelect(name string) (TUI, tea.Cmd) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}
	if cfg.SummaryModel == name {
		if name == "" {
			return t, tea.Println(hintStyle.Render("⎯ summary unchanged: (use dispatcher)") + "\n")
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ summary unchanged: %s", name)) + "\n")
	}

	cfg.SummaryModel = name
	if err := session.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}
	if name == "" {
		return t, tea.Println(hintStyle.Render("⎯ summary: (use dispatcher)") + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ summary: %s", name)) + "\n")
}

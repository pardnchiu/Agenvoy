package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type PlannerSelect struct {
	name string
}

func (t TUI) commandPlanner() (TUI, tea.Cmd, bool) {
	cfg, err := session.Load()
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
		if cfg.PlannerModel != "" && m.Name == cfg.PlannerModel {
			label += "  " + hintStyle.Render("[current]")
			cursor = i
		}
		options[i] = label
		values[i] = m.Name
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Select planner model",
		options: options,
		values:  values,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return PlannerSelect{name: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runPlannerSelect(name string) (TUI, tea.Cmd) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}
	if cfg.PlannerModel == name {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ planner unchanged: %s", name)) + "\n")
	}

	cfg.PlannerModel = name
	if err := session.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ planner: %s", name)) + "\n")
}

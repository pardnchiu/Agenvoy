package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

type ModelRemove struct {
	chosen string
}

func (t TUI) commandModelRemove() (TUI, tea.Cmd, bool) {
	cfg, err := config.Load()
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
			label += " · " + m.Description
		}
		if cfg.DispatcherModel != "" && m.Name == cfg.DispatcherModel {
			label += " · [dispatcher]"
		}
		if cfg.SummaryModel != "" && m.Name == cfg.SummaryModel {
			label += " · [summary]"
		}
		options[i] = label
		values[i] = m.Name
	}

	t.popup = &Popup{
		kind:    popupMultiSelect,
		title:   "Remove models (space toggle · enter confirm)",
		options: options,
		values:  values,
		multi:   make(map[int]bool, len(options)),
		onConfirm: func(chosen string) any {
			return ModelRemove{chosen: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runModelRemove(chosen string) (TUI, tea.Cmd) {
	if chosen == "" {
		return t, tea.Println(hintStyle.Render("⎯ no models selected") + "\n")
	}

	toRemove := make(map[string]bool)
	for _, name := range strings.Split(chosen, "\x1F") {
		if name = strings.TrimSpace(name); name != "" {
			toRemove[name] = true
		}
	}
	if len(toRemove) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no models selected") + "\n")
	}

	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}

	var kept []config.ModelEntry
	var removed []string
	for _, m := range cfg.Models {
		if toRemove[m.Name] {
			removed = append(removed, m.Name)
		} else {
			kept = append(kept, m)
		}
	}
	if len(removed) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no matching models found") + "\n")
	}

	cfg.Models = kept
	clearedDispatcher := false
	if toRemove[cfg.DispatcherModel] {
		cfg.DispatcherModel = ""
		clearedDispatcher = true
	}
	clearedSummary := false
	if toRemove[cfg.SummaryModel] {
		cfg.SummaryModel = ""
		clearedSummary = true
	}

	if err := config.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}

	agents.Reload()

	lines := []string{hintStyle.Render(fmt.Sprintf("⎯ removed: %s · registry reloaded", strings.Join(removed, ", ")))}
	if clearedDispatcher {
		lines = append(lines, warnStyle.Render("dispatcher cleared · run /model or set a new dispatcher"))
	}
	if clearedSummary {
		lines = append(lines, warnStyle.Render("summary model cleared · falls back to dispatcher"))
	}
	if len(cfg.Models) == 0 {
		lines = append(lines, warnStyle.Render("⎯ no model configured · /model global add"))
	}
	return t, tea.Println(strings.Join(lines, "\n\n") + "\n")
}

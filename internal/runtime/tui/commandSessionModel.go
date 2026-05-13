package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
)

type SessionModelSelect struct {
	model string
}

type SessionReasoningSelect struct {
	reasoning string
}

func (t TUI) commandSessionModel() (TUI, tea.Cmd, bool) {
	sid := t.currentSessionID
	if sid == "" {
		return t, tea.Println(errorStyle.Render("[!] no current session") + "\n"), true
	}

	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n"), true
	}
	if len(cfg.Models) == 0 {
		return t, tea.Println(hintStyle.Render("no models configured · use /model global > add") + "\n"), true
	}

	status := session.ReadStatus(sid)
	currentModel := status.Model
	if currentModel == "" {
		currentModel = session.StatusModel
	}

	values := make([]string, 0, len(cfg.Models)+1)
	options := make([]string, 0, len(cfg.Models)+1)
	values = append(values, session.StatusModel)
	autoLabel := session.StatusModel + "  " + hintStyle.Render("(planner picks)")
	if currentModel == session.StatusModel {
		autoLabel += "  " + hintStyle.Render("[current]")
	}

	options = append(options, autoLabel)
	cursor := 0
	for _, m := range cfg.Models {
		label := m.Name
		if m.Description != "" {
			label = fmt.Sprintf("%s  %s", m.Name, hintStyle.Render(m.Description))
		}
		if m.Name == currentModel {
			label += "  " + hintStyle.Render("[current]")
			cursor = len(values)
		}
		values = append(values, m.Name)
		options = append(options, label)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Model · session",
		options: options,
		values:  values,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return SessionModelSelect{model: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runSessionModelSelect(model string) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, tea.Println(errorStyle.Render("[!] no current session") + "\n")
	}
	session.SetModelReasoning(sid, model, "")
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ session model: %s", model)) + "\n")
}

func (t TUI) runSessionReasoningSelect(reasoning string) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, tea.Println(errorStyle.Render("[!] no current session") + "\n")
	}
	session.SetModelReasoning(sid, "", reasoning)
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ session reasoning: %s", reasoning)) + "\n")
}

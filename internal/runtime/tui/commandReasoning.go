package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

type ReasoningScopeSelect struct {
	scope string
}

type ReasoningSelect struct {
	level string
}

var reasoningLevels = []string{"low", "medium", "high"}

func (t TUI) commandReasoning(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "global":
			next, cmd := t.openReasoningGlobalPopup()
			return next, cmd, true
		case "session":
			next, cmd := t.openReasoningSessionPopup()
			return next, cmd, true
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Reasoning",
		options: []string{"global", "session"},
		values:  []string{"global", "session"},
		onConfirm: func(chosen string) any {
			return ReasoningScopeSelect{scope: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openReasoningGlobalPopup() (TUI, tea.Cmd) {
	current := provider.GetReasoningLevel()
	options := make([]string, len(reasoningLevels))
	cursor := 1
	for i, lvl := range reasoningLevels {
		label := lvl
		if lvl == current {
			label += "  " + systemStyle.Render("[current]")
			cursor = i
		}
		options[i] = label
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Reasoning · global (dispatcher)",
		options: options,
		values:  reasoningLevels,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return ReasoningSelect{level: chosen}
		},
	}
	return t, nil
}

func (t TUI) openReasoningSessionPopup() (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, tea.Println(errorStyle.Render("[!] no current session") + "\n")
	}

	_, current := configBot.GetModel(sid)

	options := make([]string, len(reasoningLevels))
	cursor := 1
	for i, lvl := range reasoningLevels {
		label := lvl
		if lvl == current {
			label += "  " + systemStyle.Render("[current]")
			cursor = i
		}
		options[i] = label
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Reasoning · session",
		options: options,
		values:  reasoningLevels,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return SessionReasoningSelect{reasoning: chosen}
		},
	}
	return t, nil
}

func (t TUI) cycleReasoning(forward bool) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, nil
	}

	_, current := configBot.GetModel(sid)
	idx := 1
	for i, lvl := range reasoningLevels {
		if lvl == current {
			idx = i
			break
		}
	}
	n := len(reasoningLevels)
	if forward {
		idx = (idx + 1) % n
	} else {
		idx = (idx - 1 + n) % n
	}
	level := reasoningLevels[idx]
	configBot.SetModel(sid, "", level)
	return t, nil
}

func (t TUI) runReasoningSelect(level string) (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)) + "\n")
	}
	if cfg.ReasoningLevel == level {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ reasoning unchanged: %s", level)) + "\n")
	}

	cfg.ReasoningLevel = level
	if err := config.Save(cfg); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)) + "\n")
	}

	provider.SetReasoningLevel(level)
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ reasoning: %s", level)) + "\n")
}

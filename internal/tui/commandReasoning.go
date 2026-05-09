package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type ReasoningSelect struct {
	level string
}

var reasoningLevels = []string{"low", "medium", "high"}

func (t TUI) commandReasoning() (TUI, tea.Cmd, bool) {
	current := provider.GetReasoningLevel()
	options := make([]string, len(reasoningLevels))
	cursor := 1
	for i, lvl := range reasoningLevels {
		label := lvl
		if lvl == current {
			label += "  " + hintStyle.Render("[current]")
			cursor = i
		}
		options[i] = label
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Select reasoning level",
		options: options,
		values:  reasoningLevels,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return ReasoningSelect{level: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runReasoningSelect(level string) (TUI, tea.Cmd) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println("\n" + errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err)))
	}
	if cfg.ReasoningLevel == level {
		return t, tea.Println("\n" + hintStyle.Render(fmt.Sprintf("⎯ reasoning unchanged: %s", level)))
	}

	cfg.ReasoningLevel = level
	if err := session.Save(cfg); err != nil {
		return t, tea.Println("\n" + errorStyle.Render(fmt.Sprintf("[!] session.Save: %v", err)))
	}

	provider.SetReasoningLevel(level)
	return t, tea.Println("\n" + hintStyle.Render(fmt.Sprintf("⎯ reasoning: %s", level)))
}

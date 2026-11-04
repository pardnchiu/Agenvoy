package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session/config"
)

type DecisionModelSelect struct {
	name string
}

func (t TUI) runDecisionModelSelect(name string) (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("config.Load: %v", err)) + "\n")
	}
	if cfg.DecisionModel == name {
		return t, nil
	}

	cfg.DecisionModel = name
	if err := config.Save(cfg); err != nil {
		return t, notice(msgError(fmt.Sprintf("config.Save: %v", err)) + "\n")
	}
	return t, nil
}

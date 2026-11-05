package tui

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

const dispatcherPrefix = "dispatch:"

type DispatcherSelect struct {
	name string
}

func dispatcherOptions() (options, values []string, cursor int) {
	cfg, err := config.Load()
	if err != nil || len(cfg.Models) == 0 {
		return nil, nil, 0
	}

	skipClaudeCode := !agenttypes.ClaudeCodeEnabled()
	for _, m := range cfg.Models {
		if skipClaudeCode && claudeCode.Is(m.Name) {
			continue
		}
		label := m.Name
		if cfg.DispatcherModel != "" && m.Name == cfg.DispatcherModel {
			label += "  " + systemStyle.Render("[current]")
			cursor = len(options)
		}
		options = append(options, label)
		values = append(values, dispatcherPrefix+m.Name)
	}
	return options, values, cursor
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
	candidates = append(candidates, configbot.DefaultModel)
	skipClaudeCode := !agenttypes.ClaudeCodeEnabled()
	for _, m := range cfg.Models {
		if skipClaudeCode && claudeCode.Is(m.Name) {
			continue
		}
		candidates = append(candidates, m.Name)
	}

	currentModel, _ := configbot.GetModel(sid)
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
	configbot.SetModel(sid, picked, "")
	return t, nil
}

func (t TUI) runDispatcherSelect(name string) (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("session.Load: %v", err)) + "\n")
	}
	if cfg.DispatcherModel == name {
		return t, nil
	}
	cfg.DispatcherModel = name
	if err := config.Save(cfg); err != nil {
		return t, notice(msgError(fmt.Sprintf("session.Save: %v", err)) + "\n")
	}
	slog.Debug("dispatcher updated", slog.String("model", name))
	return t, nil
}

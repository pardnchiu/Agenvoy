package tui

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/go-llm-router/core/claudeCode"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const sessionModelPrefix = "model:"

type SessionModelSelect struct {
	name string
}

func registeredModelOptions(sid string) (options, values []string, cursor int) {
	cfg, err := config.Load()
	if err != nil || cfg == nil || len(cfg.Models) == 0 {
		return nil, nil, 0
	}

	current := ""
	if sid != "" {
		current, _ = configbot.GetModel(sid)
	}

	options = make([]string, 0, len(cfg.Models)+1)
	values = make([]string, 0, len(cfg.Models)+1)

	auto := configbot.DefaultModel
	if current == configbot.DefaultModel {
		auto += "  " + systemStyle.Render("[current]")
	}
	options = append(options, auto)
	values = append(values, sessionModelPrefix+configbot.DefaultModel)

	skipClaudeCode := !agenttypes.ClaudeCodeEnabled()
	for _, m := range cfg.Models {
		if skipClaudeCode && claudeCode.Is(m.Name) {
			continue
		}
		label := m.Name
		if m.Name == current {
			label += "  " + systemStyle.Render("[current]")
			cursor = len(options)
		}
		if cfg.DispatcherModel != "" && m.Name == cfg.DispatcherModel {
			label += "  " + okayStyle.Render("[dispatcher]")
		}
		if cfg.SummaryModel != "" && m.Name == cfg.SummaryModel {
			label += "  " + okayStyle.Render("[summary]")
		}
		switch tag := cfg.ModelTag[m.Name]; tag {
		case "":
		case config.ModelTagPass:
			label += "  " + warnStyle.Render(tag)
		default:
			label += "  " + warnStyle.Render(tag+"-tier")
		}
		options = append(options, label)
		values = append(values, sessionModelPrefix+m.Name)
	}
	return options, values, cursor
}

func (t TUI) runSessionModelSelect(name string) (TUI, tea.Cmd) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		slog.Debug("tui: no active session")
		return t, nil
	}
	configbot.SetModel(sid, name, "")
	return t, nil
}

type ModelOrderSave struct {
	names []string
	next  any
}

func saveModelOrder(names []string) error {
	cfg, err := config.Load()
	if err != nil {
		return utilsmsg.Trace("config.Load", "", err)
	}
	var slots []int
	for i, m := range cfg.Models {
		if slices.Contains(names, m.Name) {
			slots = append(slots, i)
		}
	}
	if len(slots) != len(names) {
		return utilsmsg.New(utilsmsg.Conflict, "model list", "changed, reorder again")
	}
	for k, i := range slots {
		cfg.Models[i] = config.ModelEntry{Name: names[k]}
	}
	if err := config.Save(cfg); err != nil {
		return utilsmsg.Trace("config.Save", "", err)
	}
	agents.Reload()
	return nil
}

func (t TUI) runModelOrderSave(msg ModelOrderSave) (tea.Model, tea.Cmd) {
	if err := saveModelOrder(msg.names); err != nil {
		return t, notice(msgError(fmt.Sprintf("fallback order: %v", err)) + "\n")
	}
	if msg.next == nil {
		return t, nil
	}
	return t.update(msg.next)
}

package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

type ModelRemovePick struct {
	name string
}

type ModelRemoveConfirm struct {
	name string
	yes  bool
}

func (t TUI) openModelRemoveConfirm(name string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "Remove " + name + " ?",
		subtitle: "removed from the registry  stored credentials are kept",
		options:  []string{"No", "Yes"},
		values:   []string{"no", "yes"},
		onConfirm: func(chosen string) any {
			return ModelRemoveConfirm{name: name, yes: chosen == "yes"}
		},
	}
	return t, nil
}

func (t TUI) runModelRemove(name string) (TUI, tea.Cmd) {
	label := name
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("session.Load: %v", err)) + "\n")
	}

	var kept []config.ModelEntry
	for _, m := range cfg.Models {
		if m.Name != name {
			kept = append(kept, m)
		}
	}
	if len(kept) == len(cfg.Models) {
		slog.Debug("tui: no matching models found")
		return t, nil
	}

	cfg.Models = kept
	clearedDispatcher := false
	if cfg.DispatcherModel == name {
		cfg.DispatcherModel = ""
		clearedDispatcher = true
	}
	clearedSummary := false
	if cfg.SummaryModel == name {
		cfg.SummaryModel = ""
		clearedSummary = true
	}

	if err := config.Save(cfg); err != nil {
		return t, notice(msgError(fmt.Sprintf("session.Save: %v", err)) + "\n")
	}

	configbot.ResetModel(name)
	agents.Reload()

	slog.Debug("model removed", slog.String("model", label))
	var lines []string
	if clearedDispatcher {
		lines = append(lines, msgWarn("dispatcher cleared  run /model or set a new dispatcher"))
	}
	if clearedSummary {
		lines = append(lines, msgWarn("summary model cleared  falls back to auto"))
	}
	if len(cfg.Models) == 0 {
		lines = append(lines, msgWarn("no model configured  /model add"))
	}
	if len(lines) == 0 {
		return t, nil
	}
	return t, notice(strings.Join(lines, "\n"))
}

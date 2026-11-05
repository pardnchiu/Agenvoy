package tui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

const sessionReasoningPrefix = "reasoning:"

var reasoningLevels = configbot.ReasoningLevels()

type SessionReasoningSelect struct {
	level string
}

func currentReasoning(sid string) string {
	if sid == "" {
		return ""
	}
	_, current := configbot.GetModel(sid)
	return current
}

func reasoningOptions(sid string) (options, values []string) {
	current := currentReasoning(sid)

	options = make([]string, 0, len(reasoningLevels))
	values = make([]string, 0, len(reasoningLevels))
	for _, level := range reasoningLevels {
		label := level
		if level == current {
			label += "  " + systemStyle.Render("[current]")
		}
		options = append(options, label)
		values = append(values, sessionReasoningPrefix+level)
	}
	return options, values
}

func (t TUI) runSessionReasoningSelect(level string) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		slog.Debug("tui: no active session")
		return t, nil
	}
	configbot.SetModel(sid, "", level)
	return t, nil
}

func (t TUI) cycleReasoning(forward bool) (TUI, tea.Cmd) {
	sid := t.currentSessionID
	if sid == "" {
		return t, nil
	}

	_, current := configbot.GetModel(sid)

	idx := 0
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
	configbot.SetModel(sid, "", level)
	return t, nil
}

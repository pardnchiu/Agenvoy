package tui

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
)

type ScheduleTestPick struct {
	skill string
}

type ScheduleRemovePick struct {
	kind  string
	skill string
}

type ScheduleRemoveConfirm struct {
	kind  string
	skill string
	yes   bool
}

func (t TUI) reopenSchedule() (TUI, tea.Cmd) {
	next, cmd, _ := t.commandScheduleMenu(nil)
	return next, cmd
}

func (t TUI) runScheduleTest(skillName string) (TUI, tea.Cmd) {
	next, cmd, _ := t.commandSchedule([]string{"/sched-" + skillName})
	return next, cmd
}

func (t TUI) openScheduleRemoveConfirm(kind, skillName string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    fmt.Sprintf("Delete %s %q ?", kind, skillName),
		subtitle: "the schedule entry is dropped and its skill directory moves to Trash",
		options:  []string{"No", "Yes"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return ScheduleRemoveConfirm{kind: kind, skill: skillName, yes: chosen == "yes"}
		},
		onCancel: func() any {
			return ScheduleRemoveConfirm{kind: kind, skill: skillName}
		},
	}
	return t, nil
}

func (t TUI) runScheduleRemove(kind, skillName string) (TUI, tea.Cmd) {
	var (
		removed int
		err     error
	)
	if kind == "cron" {
		removed, err = runtime.RemoveCron(skillName)
	} else {
		removed, err = runtime.RemoveTask(skillName)
	}
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("%s remove: %v", kind, err)) + "\n")
	}
	if removed == 0 {
		slog.Debug("tui: nothing found", slog.String("kind", kind), slog.String("skill", skillName))
		return t, nil
	}
	if err := skill.TrashSchedule(context.Background(), skillName, historystore.Meta{SessionID: t.currentSessionID}); err != nil {
		return t, notice(msgError(fmt.Sprintf("TrashSchedule: %v", err)) + "\n")
	}

	slog.Debug("schedule removed", slog.String("kind", kind), slog.String("skill", skillName))
	return t.reopenSchedule()
}

func scheduleLabels(fields, names []string) (labels []string) {
	width := 0
	for _, one := range fields {
		width = max(width, len(one)+4)
	}
	labels = make([]string, len(fields))
	for i, one := range fields {
		labels[i] = padToWidth("["+one+"]", width) + names[i]
	}
	return labels
}

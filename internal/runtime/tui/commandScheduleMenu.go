package tui

import (
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (t TUI) commandScheduleMenu(_ []string) (TUI, tea.Cmd, bool) {
	labels, tails, values := t.scheduleOptions()
	if len(labels) == 0 {
		slog.Debug("tui: nothing scheduled")
		return t, nil, true
	}

	t.popup = &Popup{
		kind:       popupSingleSelect,
		title:      "/schedule",
		subtitle:   "enter fires it now  d deletes it",
		options:    labels,
		optionTail: tails,
		values:     values,
		maxVisible: cmdSelectorMaxVisible,
		cursor:     0,
		onConfirm: func(chosen string) any {
			_, name, _ := strings.Cut(chosen, ":")
			return ScheduleTestPick{skill: name}
		},
		onDelete: func(chosen string) any {
			kind, name, _ := strings.Cut(chosen, ":")
			return ScheduleRemovePick{kind: kind, skill: name}
		},
	}
	return t, nil, true
}

func (t TUI) scheduleOptions() (labels, tails, values []string) {
	crons := listCronEntries()
	tasks := listTaskEntries()

	fields := make([]string, 0, len(crons)+len(tasks))
	names := make([]string, 0, len(crons)+len(tasks))
	tails = make([]string, 0, len(crons)+len(tasks))
	values = make([]string, 0, len(crons)+len(tasks))

	add := func(field, name, sessionID, kind string) {
		fields = append(fields, field)
		names = append(names, name)
		values = append(values, kind+":"+name)
		tail := ""
		if sessionID == t.currentSessionID {
			tail = systemStyle.Render("[current]")
		}
		tails = append(tails, tail)
	}

	for _, one := range crons {
		add(one.Expression, one.Skill, one.SessionID, "cron")
	}
	for _, one := range tasks {
		add(one.At.Local().Format("2006-01-02 15:04"), one.Skill, one.SessionID, "task")
	}

	return scheduleLabels(fields, names), tails, values
}

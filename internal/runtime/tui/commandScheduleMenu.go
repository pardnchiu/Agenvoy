package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (t TUI) commandScheduleMenu(_ []string) (TUI, tea.Cmd, bool) {
	next, cmd := t.openLoading("/schedule", t.schedulePopup)
	return next, cmd, true
}

func (t TUI) schedulePopup() *Popup {
	labels, tails, values := t.scheduleOptions()
	if len(labels) == 0 {
		return messagePopup("/schedule", "nothing scheduled yet  ask the agent to add one")
	}

	return &Popup{
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

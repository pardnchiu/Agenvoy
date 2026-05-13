package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type TaskAction struct {
	action string
}

func (t TUI) commandTask(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "add":
			return t.commandTaskAdd()
		case "remove":
			return t.commandTaskRemove()
		case "edit":
			return t.commandTaskEdit()
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Task",
		options: []string{"add", "remove", "edit"},
		values:  []string{"add", "remove", "edit"},
		cursor:  0,
		onConfirm: func(chosen string) any {
			return TaskAction{action: chosen}
		},
	}
	return t, nil, true
}

func listTaskEntries() []runtime.TaskEntry {
	tasks, err := runtime.LoadTasks()
	if err != nil {
		return nil
	}
	sort.Slice(tasks, func(i, j int) bool {
		if !tasks[i].At.Equal(tasks[j].At) {
			return tasks[i].At.Before(tasks[j].At)
		}
		return tasks[i].Skill < tasks[j].Skill
	})
	return tasks
}

func taskOptions(tasks []runtime.TaskEntry) (labels []string) {
	labels = make([]string, len(tasks))
	for i, t := range tasks {
		labels[i] = t.At.Local().Format("2006-01-02 15:04") + "  " + t.Skill
	}
	return labels
}

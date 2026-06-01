package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type TaskRemoveSelect struct {
	idx int
}

type TaskRemoveConfirm struct {
	skill string
	yes   bool
}

func (t TUI) commandTaskRemove() (TUI, tea.Cmd, bool) {
	tasks := listTaskEntries()
	if len(tasks) == 0 {
		return t, tea.Println(hintStyle.Render("no tasks scheduled") + "\n"), true
	}

	labels := taskOptions(tasks)
	entries := make([]runtime.TaskEntry, len(tasks))
	copy(entries, tasks)

	values := make([]string, len(tasks))
	for i := range tasks {
		values[i] = fmt.Sprintf("%d", i)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Task remove — pick to delete",
		options: labels,
		values:  values,
		cursor:  0,
		onConfirm: func(chosen string) any {
			var idx int
			fmt.Sscanf(chosen, "%d", &idx)
			return TaskRemoveSelect{idx: idx}
		},
	}
	return t, nil, true
}

func (t TUI) openTaskRemoveConfirm(skill string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   fmt.Sprintf("Delete task %q ?", skill),
		options: []string{"No", "Yes"},
		values:  []string{"no", "yes"},
		cursor:  0,
		onConfirm: func(chosen string) any {
			return TaskRemoveConfirm{skill: skill, yes: chosen == "yes"}
		},
	}
	return t, nil
}

func (t TUI) runTaskRemove(skillName string) (TUI, tea.Cmd) {
	removed, err := runtime.RemoveTask(skillName)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] task remove: %v", err)) + "\n")
	}
	if removed == 0 {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ no task found for %s", skillName)) + "\n")
	}
	if err := skill.TrashSchedule(context.Background(), skillName); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] TrashScheduleSkill: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ removed task: %s · skill trashed", skillName)) + "\n")
}

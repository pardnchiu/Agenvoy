package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type TaskEditSelect struct {
	skill string
	at    time.Time
}

type TaskEditSubmit struct {
	skill       string
	at          time.Time
	requirement string
}

func (t TUI) commandTaskEdit() (TUI, tea.Cmd, bool) {
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
		title:   "Task edit — pick to modify",
		options: labels,
		values:  values,
		cursor:  0,
		onConfirm: func(chosen string) any {
			var idx int
			fmt.Sscanf(chosen, "%d", &idx)
			if idx < 0 || idx >= len(entries) {
				return nil
			}
			e := entries[idx]
			return TaskEditSelect{skill: e.Skill, at: e.At}
		},
	}
	return t, nil, true
}

func (t TUI) openTaskEditRequirement(skill string, at time.Time) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		title:     fmt.Sprintf("Task edit %s (current: %s) — describe change", skill, at.Local().Format("2006-01-02 15:04")),
		multiline: true,
		onConfirm: func(value string) any {
			return TaskEditSubmit{
				skill:       skill,
				at:          at,
				requirement: strings.TrimSpace(value),
			}
		},
	}
	return t, nil
}

func (t TUI) runTaskEditSubmit(skill string, at time.Time, requirement string) (TUI, tea.Cmd) {
	if requirement == "" {
		return t, tea.Println(errorStyle.Render("[!] task edit requirement required") + "\n")
	}
	prompt := fmt.Sprintf("修改一次性 task「%s」（當前觸發時間: %s）：%s\n（只改時間使用 patch_schedule(target=task)；改行為則編輯 ~/.config/agenvoy/skills/scheduler/%s/SKILL.md）",
		skill, at.Local().Format("2006-01-02 15:04"), requirement, skill)
	return t.dispatchAgent(prompt)
}

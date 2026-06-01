package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type CronRemoveSelect struct {
	skill string
}

type CronRemoveConfirm struct {
	skill string
	yes   bool
}

func (t TUI) commandCronRemove() (TUI, tea.Cmd, bool) {
	crons := listCronEntries()
	if len(crons) == 0 {
		return t, tea.Println(hintStyle.Render("no crons scheduled") + "\n"), true
	}

	labels, values := cronOptions(crons)
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Cron remove — pick to delete",
		options: labels,
		values:  values,
		cursor:  0,
		onConfirm: func(chosen string) any {
			return CronRemoveSelect{skill: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openCronRemoveConfirm(skill string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   fmt.Sprintf("Delete cron %q ?", skill),
		options: []string{"No", "Yes"},
		values:  []string{"no", "yes"},
		cursor:  0,
		onConfirm: func(chosen string) any {
			return CronRemoveConfirm{skill: skill, yes: chosen == "yes"}
		},
	}
	return t, nil
}

func (t TUI) runCronRemove(skillName string) (TUI, tea.Cmd) {
	removed, err := runtime.RemoveCron(skillName)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] cron remove: %v", err)) + "\n")
	}
	if removed == 0 {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ no cron found for %s", skillName)) + "\n")
	}
	if err := skill.TrashSchedule(context.Background(), skillName); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] TrashScheduleSkill: %v", err)) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ removed cron: %s · skill trashed", skillName)) + "\n")
}

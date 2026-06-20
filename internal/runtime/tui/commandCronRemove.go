package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type CronRemoveSelect struct {
	skills []string
}

func (t TUI) commandCronRemove() (TUI, tea.Cmd, bool) {
	crons := listCronEntries()
	if len(crons) == 0 {
		return t, tea.Println(hintStyle.Render("no crons scheduled") + "\n"), true
	}

	labels, values := t.cronOptions(crons)
	t.popup = &Popup{
		kind:    popupMultiSelect,
		title:   "Cron remove — select to delete",
		options: labels,
		values:  values,
		multi:   make(map[int]bool),
		cursor:  0,
		onConfirm: func(chosen string) any {
			if chosen == "" {
				return nil
			}
			return CronRemoveSelect{skills: strings.Split(chosen, "\x1F")}
		},
	}
	return t, nil, true
}

func (t TUI) runCronRemove(skills []string) (TUI, tea.Cmd) {
	var lines []string
	for _, skillName := range skills {
		removed, err := runtime.RemoveCron(skillName)
		if err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("[!] %s: %v", skillName, err)))
			continue
		}
		if removed == 0 {
			lines = append(lines, hintStyle.Render(fmt.Sprintf("⎯ not found: %s", skillName)))
			continue
		}
		if err := skill.TrashSchedule(context.Background(), skillName); err != nil {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("[!] %s trash: %v", skillName, err)))
			continue
		}
		lines = append(lines, hintStyle.Render(fmt.Sprintf("⎯ removed: %s", skillName)))
	}
	return t, tea.Println(strings.Join(lines, "\n") + "\n")
}

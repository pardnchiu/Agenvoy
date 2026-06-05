package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type CronEditSelect struct {
	skill      string
	expression string
}

type CronEditSubmit struct {
	skill       string
	expression  string
	requirement string
}

func (t TUI) commandCronEdit() (TUI, tea.Cmd, bool) {
	crons := listCronEntries()
	if len(crons) == 0 {
		return t, tea.Println(hintStyle.Render("no crons scheduled") + "\n"), true
	}

	labels, _ := cronOptions(crons)
	entries := make([]runtime.CronEntry, len(crons))
	copy(entries, crons)

	values := make([]string, len(crons))
	for i := range crons {
		values[i] = fmt.Sprintf("%d", i)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Cron edit — pick to modify",
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
			return CronEditSelect{skill: e.Skill, expression: e.Expression}
		},
	}
	return t, nil, true
}

func (t TUI) openCronEditRequirement(skill, expression string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		title:     fmt.Sprintf("Cron edit %s (current: %s) — describe change", skill, expression),
		multiline: true,
		onConfirm: func(value string) any {
			return CronEditSubmit{
				skill:       skill,
				expression:  expression,
				requirement: strings.TrimSpace(value),
			}
		},
	}
	return t, nil
}

func (t TUI) runCronEditSubmit(skill, expression, requirement string) (TUI, tea.Cmd) {
	if requirement == "" {
		return t, tea.Println(errorStyle.Render("[!] cron edit requirement required") + "\n")
	}
	prompt := fmt.Sprintf("修改排程「%s」（當前 cron: %s）：%s\n（只改時間使用 patch_schedule(target=cron)；改行為則編輯 ~/.config/agenvoy/skills/scheduler/%s/SKILL.md）",
		skill, expression, requirement, skill)
	return t.dispatchAgent(prompt)
}

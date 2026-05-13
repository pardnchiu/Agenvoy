package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type CronAddSubmit struct {
	requirement string
}

func (t TUI) commandCronAdd() (TUI, tea.Cmd, bool) {
	t.popup = &Popup{
		kind:      popupText,
		title:     "Cron add — describe schedule (when + what)",
		multiline: true,
		onConfirm: func(value string) any {
			return CronAddSubmit{requirement: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) runCronAddSubmit(requirement string) (TUI, tea.Cmd) {
	if requirement == "" {
		return t, tea.Println(errorStyle.Render("[!] cron requirement required") + "\n")
	}
	prompt := "/scheduler-skill-creator " + requirement
	return t.dispatchAgent(prompt)
}

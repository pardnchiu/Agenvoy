package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type TaskAddSubmit struct {
	requirement string
}

func (t TUI) commandTaskAdd() (TUI, tea.Cmd, bool) {
	t.popup = &Popup{
		kind:      popupText,
		title:     "Task add — describe one-shot task (when + what)",
		multiline: true,
		onConfirm: func(value string) any {
			return TaskAddSubmit{requirement: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) runTaskAddSubmit(requirement string) (TUI, tea.Cmd) {
	if requirement == "" {
		return t, tea.Println(errorStyle.Render("[!] task requirement required") + "\n")
	}
	prompt := "/scheduler-skill-creator " + requirement
	return t.dispatchAgent(prompt)
}

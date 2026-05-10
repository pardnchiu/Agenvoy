package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SessionSelect struct {
	id string
}

type SessionNew struct{}

func (t TUI) handleCommand(cmd string) (TUI, tea.Cmd, bool) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/exit", "/quit":
		return t, tea.Sequence(
			tea.Println("\n"+hintStyle.Render("bye.")),
			tea.Quit,
		), true

	case "/clear":
		t.tokens = 0
		t.lastIn = 0
		t.lastOut = 0
		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.discordStatus)),
		), true

	case "/switch":
		return t.commandSwitch(parts)

	case "/new":
		return t.commandNew(parts)

	case "/bot":
		return t.commandBot()

	case "/model-add":
		return t.commandModelAdd()

	case "/model-remove":
		return t.commandModelRemove()

	case "/planner":
		return t.commandPlanner()

	case "/reasoning":
		return t.commandReasoning()

	case "/session-model":
		return t.commandSessionModel()

	case "/discord-enable":
		return t.commandDiscord("enable")

	case "/discord-disable":
		return t.commandDiscord("disable")

	case "/update":
		return t.commandUpdate()
	}
	return t, nil, false
}

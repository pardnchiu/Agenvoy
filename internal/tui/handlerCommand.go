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
			tea.Println(hintStyle.Render("bye.")+"\n"),
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

	case "/model":
		return t.commandModel(parts)

	case "/planner":
		return t.commandPlanner()

	case "/reasoning":
		return t.commandReasoning(parts)

	case "/discord":
		return t.commandDiscord(parts)

	case "/update":
		return t.commandUpdate()

	case "/mode":
		return t.commandMode(parts)
	}
	return t, nil, false
}

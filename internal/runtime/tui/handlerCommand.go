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
	if strings.HasPrefix(parts[0], "/sched-") {
		return t.commandSchedule(parts)
	}
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
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		), true

	case "/switch":
		return t.commandSwitch(parts)

	case "/new":
		return t.commandNew(parts)

	case "/dangerous":
		return t.commandDangerous(parts)

	case "/reset":
		return t.commandReset()

	case "/summary":
		return t.commandSummary()

	case "/compact":
		return t.commandCompact()

	case "/bot":
		return t.commandBot(parts)

	case "/model":
		return t.commandModel(parts)

	case "/mcp":
		return t.commandMcp(parts)

	case "/discord":
		return t.commandDiscord(parts)

	case "/telegram":
		return t.commandTelegram(parts)

	case "/feature":
		return t.commandFeature(parts)

	case "/admin-channel":
		return t.commandAdminChannel(parts)

	case "/cron":
		return t.commandCron(parts)

	case "/task":
		return t.commandTask(parts)

	case "/update":
		return t.commandUpdate()

	case "/history":
		return t.commandHistory()

	case "/log":
		return t.commandLog()

	case "/cmd":
		return t.commandCmd(cmd)

	case "/key":
		return t.commandKey(parts)

	case "/pending":
		return t.commandPending()
	}
	return t, nil, false
}

func (t TUI) commandHistory() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}
	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
	}
	tail := loadSessionTail(sid)
	if len(tail) == 0 {
		seq = append(seq, tea.Println(hintStyle.Render("⎯ no history yet")+"\n"))
	} else {
		seq = append(seq, tail...)
	}
	return t, tea.Sequence(seq...), true
}

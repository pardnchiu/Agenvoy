package tui

import (
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type SessionSelect struct {
	id string
}

func (t TUI) handleCommand(cmd string) (TUI, tea.Cmd, bool) {
	parts := strings.Fields(cmd)
	if strings.HasPrefix(parts[0], "/sched-") {
		return t.commandSchedule(parts)
	}
	switch parts[0] {
	case "/exit", "/quit":
		t.notice = ""
		t.quitting = true
		return t, tea.Quit, true

	case "/clear":
		t.tokens = 0
		t.lastIn = 0
		t.lastContext = 0
		t.lastOut = 0
		t.lastCacheRead = 0
		t.lastCacheCreate = 0
		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.lineStatus, t.currentSessionID)),
		), true

	case "/session":
		return t.commandSessions(parts)

	case "/new":
		return t.commandNew(parts)

	case "/skill":
		return t.commandSkills()

	case "/compact":
		return t.commandCompactReset(1)

	case "/reset":
		return t.commandCompactReset(2)

	case "/role":
		return t.commandRole()

	case "/model":
		return t.commandModel(parts)

	case "/mcp":
		return t.commandMcp(parts)

	case "/channel":
		return t.commandChannel(parts)

	case "/config":
		return t.commandConfig()

	case "/schedule":
		return t.commandScheduleMenu(parts)

	case "/update":
		return t.commandUpdate()

	case "/resume":
		return t.commandHistory()

	case "/log":
		return t.commandLog()

	case "/usage":
		return t.commandUsage()

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
		slog.Debug("tui: no active session")
		return t, nil, true
	}
	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.lineStatus, t.currentSessionID)),
	}
	tail := loadSessionTail(sid, t.width, true)
	if len(tail) == 0 {
		slog.Debug("tui: no history yet", slog.String("session", sid))
	}
	seq = append(seq, tail...)
	return t, tea.Sequence(seq...), true
}

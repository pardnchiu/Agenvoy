package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type SessionNewSubmit struct {
	name string
}

func (t TUI) commandNew(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) >= 2 {
		name := strings.TrimSpace(strings.Join(parts[1:], " "))
		next, cmd := t.runCreateSession(name)
		return next, cmd, true
	}
	t.popup = &Popup{
		kind:  popupText,
		title: "New session name (empty = unnamed)",
		input: "",
		onConfirm: func(value string) any {
			return SessionNewSubmit{name: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) runCreateSession(name string) (TUI, tea.Cmd) {
	if name != "" {
		if owner := session.GetSessionIDByName(name); owner != "" {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] name %q already used by session %s", name, owner)) + "\n")
		}
	}

	id, err := session.CreateSession("cli-")
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] create session failed: %v", err)) + "\n")
	}

	if name != "" {
		session.SaveBot(id, name, true)
	}

	previous := t.currentSessionID
	t.currentSessionID = id
	t.currentSessionName, _ = session.GetBot(id)

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0
	t.currentModel = ""
	t.activity = ""

	if !t.onceCall {
		t = t.restartTailer()
	}

	if t.onceCall {
		return t, nil
	}

	label := utils.ShortenSessionID(id)
	if name != "" {
		label = fmt.Sprintf("%s (%s)", name, label)
	}
	lines := []string{hintStyle.Render(fmt.Sprintf("⎯ new session: %s", label))}
	if previous != "" && previous != id {
		lines = append(lines, hintStyle.Render(fmt.Sprintf("  previous: %s", utils.ShortenSessionID(previous))))
	}

	return t, tea.Sequence(
		tea.ClearScreen,
		tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		tea.Println(strings.Join(lines, "\n")+"\n"),
	)
}

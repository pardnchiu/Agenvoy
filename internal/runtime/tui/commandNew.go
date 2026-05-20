package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func (t TUI) commandNew(parts []string) (TUI, tea.Cmd, bool) {
	name := ""
	if len(parts) >= 2 {
		name = strings.TrimSpace(strings.Join(parts[1:], " "))
	}

	if name != "" {
		if owner := session.GetSessionIDByName(name); owner != "" {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] name %q already used by session %s", name, owner)) + "\n"), true
		}
	}

	id, err := session.CreateSession("cli-")
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] create session failed: %v", err)) + "\n"), true
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

	t = t.restartTailer()

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
	), true
}

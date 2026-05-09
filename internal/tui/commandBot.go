package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
)

type BotEditDone struct {
	err error
}

func (t TUI) commandBot() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println("\n" + errorStyle.Render("[!] no current session")), true
	}

	session.SaveBot(sid, sid, false)
	path := filepath.Join(filesystem.SessionsDir, sid, "bot.md")

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return t, tea.Println("\n" + errorStyle.Render("[!] EDITOR is empty")), true
	}

	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Env = os.Environ()

	return t, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return BotEditDone{err: err}
	}), true
}

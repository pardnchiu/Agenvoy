package tui

import (
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type CmdDone struct{ err error }

func (t TUI) commandCmd(raw string) (TUI, tea.Cmd, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(raw, "/cmd"))
	if rest == "" {
		return t, tea.Println(hintStyle.Render("usage: /cmd <shell command>") + "\n"), true
	}

	cmd := exec.Command("sh", "-c", rest)
	cmd.Env = os.Environ()
	cmd.Dir = t.cwd
	return t, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return CmdDone{err: err}
	}), true
}

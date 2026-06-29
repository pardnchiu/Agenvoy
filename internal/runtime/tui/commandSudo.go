package tui

import (
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/sudo"
)

type SudoAuthDone struct {
	err error
}

type sudoStream struct {
	line string
}

func (t TUI) commandSudo(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 && parts[1] == "off" {
		if !sudo.IsActive() {
			return t, tea.Println(hintStyle.Render("⎯ sudo not active") + "\n"), true
		}
		sudo.Deactivate()
		return t, tea.Println(okayStyle.Render("⎯ sudo deactivated") + "\n"), true
	}

	if sudo.IsActive() {
		remain := sudo.RemainingSeconds()
		return t, tea.Println(warnStyle.Render(fmt.Sprintf("⎯ sudo already active — %dm remaining", remain/60)) + "\n"), true
	}

	cmd := exec.Command("sudo", "-v")
	return t, tea.Sequence(
		tea.Println(warnStyle.Render("⎯ sudo: authenticating (system password required)")+"\n"),
		tea.ExecProcess(cmd, func(err error) tea.Msg {
			if err != nil {
				return SudoAuthDone{err: fmt.Errorf("authentication failed: %w", err)}
			}
			return SudoAuthDone{}
		}),
	), true
}

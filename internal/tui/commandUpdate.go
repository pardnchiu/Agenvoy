package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type UpdateConfirm struct{ ok bool }
type UpdateDone struct{ err error }

func (t TUI) commandUpdate() (TUI, tea.Cmd, bool) {
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Update agen to latest release? Will stop the daemon and exit current TUI.",
		options: []string{"Yes", "No"},
		values:  []string{"yes", "no"},
		cursor:  1,
		onConfirm: func(chosen string) any {
			return UpdateConfirm{ok: chosen == "yes"}
		},
	}
	return t, nil, true
}

func runUpdateExec() tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return UpdateDone{err: fmt.Errorf("os.Executable: %w", err)}
		}
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("%q stop && %q update", self, self))
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return UpdateDone{err: err}
	})
}

package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type UpdateConfirm struct{ ok bool }
type UpdateDone struct{ err error }

func (t TUI) commandUpdate() (TUI, tea.Cmd, bool) {
	interrupted := interactive.ListActiveTasks()
	var lines []string
	if len(interrupted) > 0 {
		lines = append(lines, warnStyle.Render(fmt.Sprintf("  %d task(s) will be interrupted:", len(interrupted))))
		for _, one := range interrupted {
			lines = append(lines, warnStyle.Render("  - "+one))
		}
	}
	t.popup = &Popup{
		kind:        popupSingleSelect,
		title:       "Update agen to latest release? Will stop the current task and the daemon, then exit current TUI.",
		styledLines: lines,
		options:     []string{"Yes", "No"},
		values:      []string{"yes", "no"},
		cursor:      1,
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
			return UpdateDone{err: utilsmsg.Wrap(utilsmsg.Internal, "os.Executable", "", err)}
		}
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("%q stop -y && %q update -y", self, self))
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return UpdateDone{err: err}
	})
}

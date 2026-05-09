package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
)

// not suitable using popup, direct using cmd
type ModelAddDone struct {
	err error
}

func (t TUI) commandModelAdd() (TUI, tea.Cmd, bool) {
	self, err := os.Executable()
	if err != nil {
		return t, tea.Println("\n" + errorStyle.Render(fmt.Sprintf("[!] os.Executable: %v", err))), true
	}

	cmd := exec.Command(self, "model", "add")
	cmd.Env = os.Environ()

	exec := tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return ModelAddDone{err: err}
		}
		return ModelAddDone{}
	})

	return t, tea.Sequence(
		tea.Println("\n"+hintStyle.Render("⎯ launching add-model flow · ctrl+c to cancel")),
		exec,
	), true
}

func (t TUI) commandModelList() (TUI, tea.Cmd, bool) {
	cfg, err := session.Load()
	if err != nil {
		return t, tea.Println("\n" + errorStyle.Render(fmt.Sprintf("[!] session.Load: %v", err))), true
	}

	if len(cfg.Models) == 0 {
		return t, tea.Println("\n" + hintStyle.Render("no models configured · use /model-add")), true
	}

	lines := make([]string, 0, len(cfg.Models)*2+1)
	lines = append(lines, hintStyle.Render(fmt.Sprintf("⎯ %d model(s)", len(cfg.Models))))
	for _, m := range cfg.Models {
		row := "  " + textStyle.Render(m.Name)
		if cfg.PlannerModel != "" && m.Name == cfg.PlannerModel {
			row += " " + textStyle.Render("(planner)")
		}
		if m.Description != "" {
			row += "  " + hintStyle.Render(m.Description)
		}
		lines = append(lines, row)
	}
	return t, tea.Println("\n" + strings.Join(lines, "\n")), true
}

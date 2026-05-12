package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
)

type DiscordAction struct {
	action string
}

type DiscordDone struct {
	action string
	err    error
}

func (t TUI) commandDiscord(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable", "disable":
			action := parts[1]
			return t, func() tea.Msg { return DiscordAction{action: action} }, true
		}
	}

	enabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		enabled = cfg.DiscordEnabled
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Discord",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return DiscordAction{action: chosen}
		},
	}
	return t, nil, true
}

func runDiscordAction(action string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return tea.Println(errorStyle.Render(fmt.Sprintf("[!] os.Executable: %v", err)) + "\n")
	}

	cmd := exec.Command(self, "discord", action)
	cmd.Env = os.Environ()

	execCmd := tea.ExecProcess(cmd, func(err error) tea.Msg {
		return DiscordDone{action: action, err: err}
	})

	return tea.Sequence(
		tea.Println(hintStyle.Render(fmt.Sprintf("⎯ discord %s · ctrl+c to cancel", action))+"\n"),
		execCmd,
	)
}

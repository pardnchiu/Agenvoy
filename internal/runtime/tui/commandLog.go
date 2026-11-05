package tui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type LogDone struct{ err error }

func (t TUI) commandLog() (TUI, tea.Cmd, bool) {
	path := filesystem.DaemonLogPath
	if !go_pkg_filesystem_reader.Exists(path) {
		slog.Debug("tui: no daemon log yet")
		return t, nil, true
	}

	pager := strings.TrimSpace(os.Getenv("PAGER"))
	if pager == "" {
		pager = "less -Rf +F"
	}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %q", pager, path))
	cmd.Env = os.Environ()
	return t, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return LogDone{err: err}
	}), true
}

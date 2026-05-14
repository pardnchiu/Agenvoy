package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type LogDone struct{ err error }

func (t TUI) commandLog() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}
	path := filepath.Join(filesystem.SessionsDir, sid, "action.log")
	if !go_pkg_filesystem_reader.Exists(path) {
		return t, tea.Println(hintStyle.Render("⎯ no log yet") + "\n"), true
	}

	pager := strings.TrimSpace(os.Getenv("PAGER"))
	if pager == "" {
		pager = "less -Rf +G"
	}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("tr '\\037' '\\n' < %q | %s", path, pager))
	cmd.Env = os.Environ()
	return t, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return LogDone{err: err}
	}), true
}

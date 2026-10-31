package tui

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type WorkDirSubmit struct {
	value string
}

func (t TUI) commandWorkDir() TUI {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Work directory",
		subtitle: "absolute, ~/ or relative to now " + t.cwd,
		input:    newPopupInput(t.cwd, false),
		onConfirm: func(value string) any {
			return WorkDirSubmit{value: strings.TrimSpace(value)}
		},
	}
	return t
}

func (t TUI) runWorkDirSubmit(value string) (TUI, tea.Cmd) {
	if value == "" {
		return t, notice(msgWarn("work directory: empty path") + "\n")
	}
	abs := go_pkg_utils.AbsPath(t.cwd, value)
	if !go_pkg_filesystem_reader.IsDir(abs) {
		return t, notice(msgError(fmt.Sprintf("[Not found] work directory: %s", abs)) + "\n")
	}
	abs = actualCase(abs)
	t.cwd = abs
	slog.Debug("work directory updated", slog.String("dir", abs))
	return t, nil
}

func actualCase(path string) string {
	resolved := string(filepath.Separator)
	for part := range strings.SplitSeq(strings.TrimPrefix(path, resolved), string(filepath.Separator)) {
		name := part
		if dirs, err := go_pkg_filesystem_reader.ListDirs(resolved); err == nil {
			for _, d := range dirs {
				if d.Name == part {
					name = part
					break
				}
				if strings.EqualFold(d.Name, part) {
					name = d.Name
				}
			}
		}
		resolved = filepath.Join(resolved, name)
	}
	return resolved
}

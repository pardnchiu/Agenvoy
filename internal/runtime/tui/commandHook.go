package tui

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
)

type HookOpenPick struct {
	path string
}

type HookRemovePick struct {
	path string
}

func (t TUI) commandHook() (TUI, tea.Cmd, bool) {
	next, cmd := t.openLoading("/hook", hookPopup)
	return next, cmd, true
}

func hookPopup() *Popup {
	var labels, tails, values []string
	for _, name := range agentsexechook.Names() {
		py := filepath.Join(filesystem.HooksDir, name+".py")
		js := filepath.Join(filesystem.HooksDir, name+".js")
		hasPy := go_pkg_filesystem_reader.IsFile(py)
		hasJs := go_pkg_filesystem_reader.IsFile(js)
		if hasPy {
			tail := ""
			if hasJs {
				tail = systemStyle.Render("(primary)")
			}
			labels = append(labels, name+".py")
			tails = append(tails, tail)
			values = append(values, py)
		}
		if hasJs {
			labels = append(labels, name+".js")
			tails = append(tails, "")
			values = append(values, js)
		}
	}
	if len(labels) == 0 {
		return &Popup{
			kind:        popupSingleSelect,
			title:       "/hook",
			styledLines: []string{hintStyle.Render("  no hook added yet  ask the agent to add one")},
			options:     []string{filesystem.HooksDir},
			values:      []string{filesystem.HooksDir},
			enterAction: "open folder",
			onConfirm: func(chosen string) any {
				return HookOpenPick{path: chosen}
			},
		}
	}

	return &Popup{
		kind:        popupSingleSelect,
		title:       "/hook",
		subtitle:    "enter opens it  d moves it to Trash",
		options:     labels,
		optionTail:  tails,
		values:      values,
		maxVisible:  cmdSelectorMaxVisible,
		cursor:      0,
		enterAction: "open file",
		onConfirm: func(chosen string) any {
			return HookOpenPick{path: chosen}
		},
		onDelete: func(chosen string) any {
			return HookRemovePick{path: chosen}
		},
	}
}

func openHook(path string) tea.Cmd {
	if path == filesystem.HooksDir {
		if err := go_pkg_filesystem.CheckDir(path, true); err != nil {
			return notice(msgError(fmt.Sprintf("hook dir: %v", err)) + "\n")
		}
	}
	openBrowser(path)
	return nil
}

func (t TUI) runHookRemove(path string) (TUI, tea.Cmd) {
	if !go_pkg_filesystem_reader.IsFile(path) {
		next, cmd, _ := t.commandHook()
		return next, cmd
	}
	trashPath, err := filesystem.TrashDir(path, filesystem.HookTrashDir, filepath.Base(path))
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("hook remove: %v", err)) + "\n")
	}
	if err := historystore.RecordDelete(context.Background(), path, trashPath, historystore.Meta{SessionID: t.currentSessionID, Tool: "hook"}); err != nil {
		slog.Debug("historystore.RecordDelete",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
	next, cmd, _ := t.commandHook()
	return next, cmd
}

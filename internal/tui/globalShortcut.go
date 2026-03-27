package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/rivo/tview"
)

func globalShortcut(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		app.Stop()
		return nil

	case tcell.KeyEsc:
		resetView()
		return nil

	case tcell.KeyRight:
		if isPopup() {
			return event
		}
		focusIndex = (focusIndex + 1) % len(panels)
		app.SetFocus(panels[focusIndex])
		return nil

	case tcell.KeyLeft:
		if isPopup() {
			return event
		}
		focusIndex = (focusIndex + len(panels) - 1) % len(panels)
		app.SetFocus(panels[focusIndex])
		return nil
	}

	switch event.Rune() {
	case 'q':
		app.Stop()
		return nil

	case 'e':
		if openEditor() {
			return nil
		}

	case 'd':
		if deleteFile() {
			return nil
		}
	}
	return event
}

func resetView() {
	if currentDir != filesystem.AgenvoyDir {
		currentDir = filesystem.AgenvoyDir
		go app.QueueUpdateDraw(func() {
			loadDir(filesView, currentDir)
		})
	}

	if currentPath != "" {
		currentPath = ""
		go app.QueueUpdateDraw(func() {
			contentView.SetTitle(" Content ")
			contentView.SetText(setDefault())
			contentView.ScrollToBeginning()
		})
	}
}

func isPopup() bool {
	focused := app.GetFocus()
	if slices.Contains(panels, focused) {
		return false
	}
	return true
}

func openEditor() bool {
	var filrPath string

	switch app.GetFocus() {
	case filesView:
		idx := filesView.GetCurrentItem()
		if idx >= 0 && idx < len(flieLists) {
			name := flieLists[idx]
			if name != "../" && !strings.HasSuffix(name, "/") {
				filrPath = filepath.Join(currentDir, name)
			}
		}

	case contentView:
		filrPath = currentPath
	}

	if filrPath != "" {
		// * because i save with minify formať, so pretty it first
		if filepath.Ext(filrPath) == ".json" {
			if raw, err := os.ReadFile(filrPath); err == nil {
				var buf bytes.Buffer
				if json.Indent(&buf, raw, "", "  ") == nil {
					_ = os.WriteFile(filrPath, buf.Bytes(), 0644)
				}
			}
		}

		app.Suspend(func() {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			cmd := exec.Command(editor, filrPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				slog.Warn("cmd.Run",
					slog.String("error", err.Error()))
			}
		})

		if filrPath == currentPath {
			go app.QueueUpdateDraw(func() {
				contentView.SetText(readFile(currentPath))
				contentView.ScrollToBeginning()
			})
		}
		return true
	}
	return false
}

func deleteFile() bool {
	if app.GetFocus() != filesView {
		return false
	}

	index := filesView.GetCurrentItem()
	if index < 0 || index >= len(flieLists) {
		return false
	}

	name := flieLists[index]
	if name == "../" {
		return false
	}

	target := filepath.Join(currentDir, name)
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Delete %s ?", name)).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, _ string) {
			pages.RemovePage("confirm-deleted")
			app.SetFocus(filesView)
			if buttonIndex != 0 {
				return
			}

			info, err := os.Stat(target)
			if err == nil && info.IsDir() {
				err = os.RemoveAll(target)
			} else {
				err = os.Remove(target)
			}
			if err != nil {
				slog.Warn("os.RemoveAll/os.Remove",
					slog.String("error", err.Error()))
				return
			}

			go app.QueueUpdateDraw(func() {
				loadDir(filesView, currentDir)
				if currentPath == target {
					currentPath = ""
					contentView.SetTitle(" Content ")
					contentView.SetText(setDefault())
					contentView.ScrollToBeginning()
				}
			})
		})
	pages.AddPage("confirm-deleted", modal, true, true)
	app.SetFocus(modal)
	return true
}

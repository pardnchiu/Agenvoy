package tui

import (
	"github.com/gdamore/tcell/v2"
	go_pkg_tui "github.com/pardnchiu/go-pkg/tui"
	"github.com/rivo/tview"
)

func newMainView() tview.Primitive {
	filesView = go_pkg_tui.NewList(&go_pkg_tui.List{
		MainTextColor: tcell.ColorWhite,
		Border: &go_pkg_tui.Border{
			Title:      "Files",
			TitleAlign: tview.AlignLeft,
			FocusColor: tcell.ColorYellow,
		},
	})
	loadDir(filesView, currentDir)
	filesView.SetSelectedFunc(selectFile)

	contentView = go_pkg_tui.NewTextView(&go_pkg_tui.TextView{
		Scrollable:    true,
		Wrap:          true,
		WordWrap:      true,
		DynamicColors: true,
		Border: &go_pkg_tui.Border{
			Title:      "Content",
			TitleAlign: tview.AlignLeft,
			FocusColor: tcell.ColorYellow,
		},
	})
	contentView.SetText(setDefault())

	logsView = go_pkg_tui.NewTextView(&go_pkg_tui.TextView{
		Scrollable:    true,
		Wrap:          true,
		WordWrap:      true,
		DynamicColors: true,
		Border: &go_pkg_tui.Border{
			Title:      "Logs",
			TitleAlign: tview.AlignLeft,
			FocusColor: tcell.ColorYellow,
		},
	})

	viewPages = tview.NewPages().
		AddPage("content", contentView, true, true).
		AddPage("logs", logsView, true, false)

	return tview.NewFlex().
		AddItem(filesView, 0, 1, true).
		AddItem(viewPages, 0, 4, false)
}

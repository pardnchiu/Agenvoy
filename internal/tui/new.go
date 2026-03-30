package tui

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/rivo/tview"
)

const (
	projectVersion = "v0.17.2"
)

type tuiWriter struct {
	app  *tview.Application
	view *tview.TextView
}

var (
	once        sync.Once
	app         *tview.Application
	layout      *tview.Flex
	pages       *tview.Pages
	filesView   *tview.List
	contentView *tview.TextView
	logsView    *tview.TextView
	panels      []tview.Primitive
	focusIndex  int
	currentDir  string
	flieLists   []string
	currentPath string
)

func New() {
	once.Do(func() {
		currentDir = filesystem.AgenvoyDir

		app = tview.NewApplication()

		filesView = tview.NewList().
			ShowSecondaryText(false).
			SetMainTextColor(tcell.ColorWhite)
		filesView.SetBorder(true).
			SetTitle(" Files ").
			SetTitleAlign(tview.AlignLeft)
		loadDir(filesView, currentDir)

		contentView = tview.NewTextView().
			SetScrollable(true).
			SetWrap(true).
			SetDynamicColors(true)
		contentView.SetBorder(true).
			SetTitle(" Content ").
			SetTitleAlign(tview.AlignLeft)
		contentView.SetText(setDefault())

		logsView = tview.NewTextView().
			SetScrollable(true).
			SetWrap(true)
		logsView.SetBorder(true).
			SetTitle(" Logs ").
			SetTitleAlign(tview.AlignLeft)

		panels = []tview.Primitive{filesView, contentView, logsView}
		for _, p := range panels {
			box := p.(interface {
				SetBorderColor(tcell.Color) *tview.Box
				SetFocusFunc(func()) *tview.Box
				SetBlurFunc(func()) *tview.Box
			})
			box.SetFocusFunc(func() { box.SetBorderColor(tcell.ColorYellow) })
			box.SetBlurFunc(func() { box.SetBorderColor(tcell.ColorWhite) })
		}

		mainFlex := tview.NewFlex().
			AddItem(filesView, 0, 1, true).
			AddItem(contentView, 0, 2, false).
			AddItem(logsView, 0, 2, false)

		pages = tview.NewPages().
			AddPage("main", mainFlex, true, true)

		layout = tview.NewFlex().
			AddItem(pages, 0, 1, true)

		go fetchMeta()

		filesView.SetSelectedFunc(selectFile)
		app.SetInputCapture(globalShortcut)
	})
}

func SetSlog() {
	slog.SetDefault(slog.New(slog.NewTextHandler(
		&tuiWriter{
			app:  app,
			view: logsView,
		}, nil,
	)))
}

// * extended functions
func (w *tuiWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	go w.app.QueueUpdateDraw(func() {
		fmt.Fprint(w.view, text)
		w.view.ScrollToEnd()
	})
	return len(p), nil
}

func Set() error {
	return app.SetRoot(layout, true).Run()
}

func Stop() {
	app.Stop()
}

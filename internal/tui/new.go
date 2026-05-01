package tui

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/rivo/tview"
)

var projectVersion = "dev"

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
	viewPages   *tview.Pages
	panels      []tview.Primitive
	focusIndex  int
	currentDir  string
	flieLists   []string
	currentPath string

	cmdInput  *tview.InputField
	isMsgMode bool
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
			SetWrap(true).
			SetDynamicColors(true)
		logsView.SetBorder(true).
			SetTitle(" Logs ").
			SetTitleAlign(tview.AlignLeft)

		viewPages = tview.NewPages().
			AddPage("content", contentView, true, true).
			AddPage("logs", logsView, true, false)

		panels = []tview.Primitive{filesView, viewPages}
		borderTargets := []tview.Primitive{filesView, contentView, logsView}
		for _, p := range borderTargets {
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
			AddItem(viewPages, 0, 4, false)

		cmdInput = tview.NewInputField().
			SetLabel("$ ").
			SetFieldBackgroundColor(tcell.ColorDefault).
			SetLabelColor(tcell.ColorYellow)
		cmdInput.SetBorder(true).
			SetTitle(" Command ").
			SetTitleAlign(tview.AlignLeft)
		cmdInput.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				text := cmdInput.GetText()
				cmdInput.SetText("")
				if isMsgMode {
					executeMessage(text)
				} else {
					executeCommand(text)
				}
			case tcell.KeyEscape:
				hideCommandInput()
			}
		})
		cmdInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab {
				toggleInputMode()
				return nil
			}
			return event
		})

		pages = tview.NewPages().
			AddPage("main", mainFlex, true, true)

		layout = tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(pages, 0, 1, true).
			AddItem(cmdInput, 0, 0, false)

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

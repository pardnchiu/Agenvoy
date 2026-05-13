package tui

import (
	"context"
	"fmt"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/agenvoy/internal/tools"
)

var (
	program atomic.Pointer[tea.Program]

	colSystem = lipgloss.Color("75")  // sky blue
	colHint   = lipgloss.Color("240") // gray
	colWarn   = lipgloss.Color("141") // purple
	colOk     = lipgloss.Color("114") // green
	colSkill  = lipgloss.Color("208") // orange
	colError  = lipgloss.Color("203") // red

	systemStyle = lipgloss.NewStyle().Foreground(colSystem)
	okayStyle   = lipgloss.NewStyle().Foreground(colOk)
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	skillStyle  = lipgloss.NewStyle().Foreground(colSkill)
	hintStyle   = lipgloss.NewStyle().Foreground(colHint)
	errorStyle  = lipgloss.NewStyle().Foreground(colError)
	textStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	whiteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

type WorkDir struct {
	dir string
}

func Run(ctx context.Context) error {
	prog := tea.NewProgram(newModel(ctx), tea.WithContext(ctx))
	program.Store(prog)
	defer program.Store(nil)

	tools.WorkDirChangeHook = func(dir string) {
		send(WorkDir{dir: dir})
	}
	defer func() {
		tools.WorkDirChangeHook = nil
	}()

	go newPendingChannel(ctx)
	go fetchProjectRelease(ctx)

	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("prog.Run: %w", err)
	}
	return nil
}

func send(msg tea.Msg) {
	if prog := program.Load(); prog != nil {
		prog.Send(msg)
	}
}

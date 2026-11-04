package tui

import (
	"context"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/ipc"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var (
	program   atomic.Pointer[tea.Program]
	ipcClient atomic.Pointer[ipc.Client]

	colSystem = lipgloss.AdaptiveColor{Light: "#005FAF", Dark: "#5FAFFF"} // sky blue
	colWarn   = lipgloss.AdaptiveColor{Light: "#5F3DAF", Dark: "#875FD7"} // purple
	colOk     = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#5FAF5F"} // green
	colSkill  = lipgloss.AdaptiveColor{Light: "#AF5700", Dark: "#FF8700"} // orange
	colError  = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF5F5F"} // red
	colHint   = lipgloss.AdaptiveColor{Light: "#626262", Dark: "#626262"} // gray 0
	colText   = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#8A8A8A"} // gray 1
	colThink  = lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#AAAAAA"}
	colCursor = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}

	systemStyle = lipgloss.NewStyle().Foreground(colSystem)
	okayStyle   = lipgloss.NewStyle().Foreground(colOk)
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	skillStyle  = lipgloss.NewStyle().Foreground(colSkill)
	hintStyle   = lipgloss.NewStyle().Foreground(colHint)
	errorStyle  = lipgloss.NewStyle().Foreground(colError)
	textStyle   = lipgloss.NewStyle().Foreground(colText)
	userStyle   = lipgloss.NewStyle().Foreground(colSkill)
	whiteStyle  = lipgloss.NewStyle().Foreground(colCursor)
	thinkStyle  = lipgloss.NewStyle().Foreground(colThink)
	keyStyle    = lipgloss.NewStyle().Foreground(colThink)
)

type daemonState struct {
	connected bool
}

type restrictedVerified struct {
	err error
}

func Run(ctx context.Context) error {
	prog := tea.NewProgram(newModel(ctx), tea.WithContext(ctx), tea.WithoutSignalHandler())
	program.Store(prog)
	defer program.Store(nil)

	restoreSlog := installSlogTUI()
	defer restoreSlog()

	ipcClient.Store(ipc.Connect(ctx, func(f ipc.Frame) {
		send(Pending{
			id: f.Ask.ID,
			request: runtime.Request{
				ID:         f.Ask.ID,
				Kind:       f.Ask.Kind,
				SessionID:  f.SessionID,
				DeliverTo:  f.SessionID,
				ToolName:   f.Ask.ToolName,
				ToolArgs:   f.Ask.ToolArgs,
				Restricted: f.Ask.Restricted,
				AskUser:    &runtime.UserPayload{Questions: f.Ask.Questions},
				Inline:     true,
			},
			needPassword: f.Ask.NeedPassword,
		})
	}, func(err error) {
		send(restrictedVerified{err: err})
	}, func(connected bool) {
		send(daemonState{connected: connected})
	}))

	go newDaemonLog(ctx)

	if _, err := prog.Run(); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "prog.Run", "", err)
	}
	return nil
}

func send(msg tea.Msg) {
	if prog := program.Load(); prog != nil {
		prog.Send(msg)
	}
}

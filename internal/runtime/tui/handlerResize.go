package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const resizeSettleDelay = 250 * time.Millisecond

type resizeSettled struct {
	gen int
}

func resizeSettleTick(gen int) tea.Cmd {
	return tea.Tick(resizeSettleDelay, func(time.Time) tea.Msg {
		return resizeSettled{gen: gen}
	})
}

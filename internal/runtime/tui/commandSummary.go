package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type SummaryDone struct {
	id    string
	count int
	err   error
}

func (t TUI) commandSummary() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}

	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = utils.ShortenSessionID(sid)
	t.activity = "regenerating summary…"

	return t, tea.Batch(
		tea.Println(hintStyle.Render(fmt.Sprintf("⎯ refreshing summary for %s…", utils.ShortenSessionID(sid)))+"\n"),
		t.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			count, err := exec.ForceSummary(ctx, sid)
			return SummaryDone{id: sid, count: count, err: err}
		},
	), true
}

func (t TUI) finishSummary(msg SummaryDone) (TUI, tea.Cmd) {
	t.running = false
	t.activity = ""
	t.runTarget = ""

	if msg.err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] summary failed: %v", msg.err)) + "\n")
	}
	if msg.count == 0 {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ summary: %s (nothing new to summarize)", utils.ShortenSessionID(msg.id))) + "\n")
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ summary: %s (%d messages processed)", utils.ShortenSessionID(msg.id), msg.count)) + "\n")
}

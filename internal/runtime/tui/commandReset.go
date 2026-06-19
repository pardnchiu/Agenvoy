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

type ResetSessionConfirm1 struct {
	id  string
	yes bool
}

type ResetSessionConfirm2 struct {
	id  string
	yes bool
}

func (t TUI) commandReset() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}

	label := utils.ShortenSessionID(sid)
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    fmt.Sprintf("Reset history for %s ?", label),
		subtitle: "Summary will be regenerated first; bot identity and summary are kept.",
		options:  []string{"No", "Yes"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return ResetSessionConfirm1{id: sid, yes: chosen == "yes"}
		},
	}
	return t, nil, true
}

func (t TUI) openResetConfirm2(sid string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "Are you sure? Raw history will be permanently dropped.",
		subtitle: fmt.Sprintf("%s — summary refresh runs first; abort if refresh fails.", utils.ShortenSessionID(sid)),
		options:  []string{"No", "Yes  reset it"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return ResetSessionConfirm2{id: sid, yes: chosen == "yes"}
		},
	}
	return t, nil
}

type ResetSessionDone struct {
	id   string
	keys int
	err  error
}

func (t TUI) runResetSession(sid string) (TUI, tea.Cmd) {
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = utils.ShortenSessionID(sid)
	t.activity = "resetting (summary refresh first)…"

	return t, tea.Batch(
		tea.Println(hintStyle.Render(fmt.Sprintf("⎯ refreshing summary for %s, then clearing history…", utils.ShortenSessionID(sid)))+"\n"),
		t.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			keys, err := exec.ResetSessionWithSummary(ctx, sid)
			return ResetSessionDone{id: sid, keys: keys, err: err}
		},
	)
}

func (t TUI) finishResetSession(msg ResetSessionDone) (TUI, tea.Cmd) {
	t.running = false
	t.activity = ""
	t.runTarget = ""

	if msg.err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] reset failed: %v", msg.err)) + "\n")
	}

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		tea.Println(hintStyle.Render(fmt.Sprintf("⎯ reset: %s (summary kept, %d torii keys purged)", utils.ShortenSessionID(msg.id), msg.keys)) + "\n"),
	}
	return t, tea.Sequence(seq...)
}

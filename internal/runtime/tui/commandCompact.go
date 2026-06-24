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

type CompactConfirm struct {
	id  string
	yes bool
}

type CompactDone struct {
	id      string
	removed int
	err     error
}

func (t TUI) commandCompact() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}

	label := utils.ShortenSessionID(sid)
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    fmt.Sprintf("Compact history for %s ?", label),
		subtitle: "Redundant and meaningless exchanges will be removed by LLM analysis.",
		options:  []string{"No", "Yes"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return CompactConfirm{id: sid, yes: chosen == "yes"}
		},
	}
	return t, nil, true
}

func (t TUI) runCompact(sid string) (TUI, tea.Cmd) {
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = utils.ShortenSessionID(sid)
	t.activity = "compacting history…"

	return t, tea.Batch(
		tea.Println(hintStyle.Render(fmt.Sprintf("⎯ compacting history for %s…", utils.ShortenSessionID(sid)))+"\n"),
		t.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			removed, err := exec.CompactHistory(ctx, sid)
			return CompactDone{id: sid, removed: removed, err: err}
		},
	)
}

func (t TUI) finishCompact(msg CompactDone) (TUI, tea.Cmd) {
	t.running = false
	t.activity = ""
	t.runTarget = ""

	if msg.err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] compact failed: %v", msg.err)) + "\n")
	}

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0

	hint := fmt.Sprintf("⎯ compact: %s (nothing to remove)", utils.ShortenSessionID(msg.id))
	if msg.removed > 0 {
		hint = fmt.Sprintf("⎯ compact: %s (%d messages removed)", utils.ShortenSessionID(msg.id), msg.removed)
	}

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
	}
	tail := loadSessionTail(msg.id)
	if len(tail) == 0 {
		seq = append(seq, tea.Println(hintStyle.Render("⎯ no history yet")+"\n"))
	} else {
		seq = append(seq, tail...)
	}
	seq = append(seq, tea.Println(hintStyle.Render(hint)+"\n"))
	return t, tea.Sequence(seq...)
}

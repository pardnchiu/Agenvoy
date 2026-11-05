package tui

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type ResetSessionConfirm1 struct {
	id   string
	mode string
}

type ResetSessionConfirm2 struct {
	id   string
	mode string
	yes  bool
}

func (t TUI) openResetConfirm2(sid, mode string) (TUI, tea.Cmd) {
	title := "Are you sure? Raw history will be permanently dropped."
	subtitle := fmt.Sprintf("%s — summary refresh runs first; abort if refresh fails.", utils.ShortenSessionID(sid))
	if mode == "all" {
		title = "Are you sure? History AND summary will be permanently dropped."
		subtitle = fmt.Sprintf("%s — no refresh; this session's memory is fully wiped.", utils.ShortenSessionID(sid))
	}

	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    title,
		subtitle: subtitle,
		options:  []string{"No", "Yes  reset it"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return ResetSessionConfirm2{id: sid, mode: mode, yes: chosen == "yes"}
		},
	}
	return t, nil
}

type ResetSessionDone struct {
	id   string
	mode string
	keys int
	err  error
}

func (t TUI) runResetSession(sid, mode string) (TUI, tea.Cmd) {
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = utils.ShortenSessionID(sid)

	label := utils.ShortenSessionID(sid)
	if mode == "all" {
		t.activity = "resetting (history + summary)..."
		slog.Debug("clearing history and summary",
			slog.String("session", label))
		return t, tea.Batch(
			t.spinner.Tick,
			func() tea.Msg {
				keys, err := exec.Reset(context.Background(), sid, false)
				return ResetSessionDone{id: sid, mode: mode, keys: keys, err: err}
			},
		)
	}

	t.activity = "resetting (summary refresh first)..."
	slog.Debug("tui: refreshing summary before clearing history", slog.String("session", sid))
	return t, tea.Batch(
		t.spinner.Tick,
		func() tea.Msg {
			ctx := context.Background()
			keys, err := exec.Reset(ctx, sid, true)
			return ResetSessionDone{id: sid, mode: mode, keys: keys, err: err}
		},
	)
}

func (t TUI) finishResetSession(msg ResetSessionDone) (TUI, tea.Cmd) {
	t.running = false
	t.activity = ""
	t.runTarget = ""

	if msg.err != nil {
		return t, notice(msgError(fmt.Sprintf("reset failed: %v", msg.err)) + "\n")
	}

	t.tokens = 0
	t.lastIn = 0
	t.lastContext = 0
	t.lastOut = 0
	t.lastCacheRead = 0
	t.lastCacheCreate = 0

	summaryNote := "summary kept"
	if msg.mode == "all" {
		summaryNote = "summary cleared"
	}

	slog.Debug("reset",
		slog.String("session", utils.ShortenSessionID(msg.id)),
		slog.String("summary", summaryNote),
		slog.Int("torii_keys_purged", msg.keys))
	return t, tea.Sequence(
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.lineStatus, t.currentSessionID)),
	)
}

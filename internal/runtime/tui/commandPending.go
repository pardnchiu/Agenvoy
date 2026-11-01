package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func noPendingNotice() tea.Cmd {
	return notice(msgLog("no pending task  it may have been resumed or deleted") + "\n")
}

type PendingSelect struct {
	id       string
	taskHash string
}

type PendingDeletePick struct {
	id       string
	taskHash string
	label    string
}

type PendingDeleteConfirm struct {
	id       string
	taskHash string
	label    string
	yes      bool
}

func (t TUI) commandPending() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		slog.Debug("tui: no active session")
		return t, nil, true
	}

	hashes := interactive.ListResumablePending(sid)
	if len(hashes) == 0 {
		return t, noPendingNotice(), true
	}

	options := make([]string, 0, len(hashes))
	values := make([]string, 0, len(hashes))
	tails := make([]string, 0, len(hashes))
	locked := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		info, ok := interactive.LoadPendingInfo(sid, h)
		if !ok {
			continue
		}
		label := go_pkg_utils.TruncateString(h, 8)
		if info.Objective != "" {
			label = go_pkg_utils.TruncateString(info.Objective, 64)
		}
		if !info.UpdatedAt.IsZero() {
			label = info.UpdatedAt.Local().Format("01-02 15:04") + "  " + label
		}
		if info.HasQuestions {
			label += " (awaiting answer)"
		}
		tail := ""
		if running, _ := interactive.BackgroundState(sid, h); running > 0 {
			tail = okayStyle.Render("[Active]")
			locked[h] = true
		}
		options = append(options, label)
		tails = append(tails, tail)
		values = append(values, h)
	}

	if len(options) == 0 {
		return t, noPendingNotice(), true
	}

	sessionID := sid
	labels := make(map[string]string, len(values))
	for i, h := range values {
		labels[h] = options[i]
	}

	t.popup = &Popup{
		kind:       popupSingleSelect,
		title:      "/pending",
		subtitle:   fmt.Sprintf("%d pending task(s)", len(options)),
		options:    options,
		values:     values,
		optionTail: tails,
		maxVisible: cmdSelectorMaxVisible,
		onConfirm: func(chosen string) any {
			return PendingSelect{id: sessionID, taskHash: chosen}
		},
		onDelete: func(chosen string) any {
			if locked[chosen] {
				return nil
			}
			return PendingDeletePick{id: sessionID, taskHash: chosen, label: labels[chosen]}
		},
	}
	return t, nil, true
}

func (t TUI) openPendingDeleteConfirm(msg PendingDeletePick) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "Drop pending task ?",
		subtitle: msg.label,
		options:  []string{"No", "Yes"},
		values:   []string{"no", "yes"},
		onConfirm: func(chosen string) any {
			return PendingDeleteConfirm{id: msg.id, taskHash: msg.taskHash, label: msg.label, yes: chosen == "yes"}
		},
		onCancel: func() any {
			return PendingDeleteConfirm{id: msg.id, taskHash: msg.taskHash, label: msg.label}
		},
	}
	return t, nil
}

func (t TUI) runPendingDelete(msg PendingDeleteConfirm) (TUI, tea.Cmd) {
	if _, ok := interactive.LoadPendingInfo(msg.id, msg.taskHash); !ok {
		next, cmd, _ := t.commandPending()
		return next, tea.Sequence(noPendingNotice(), cmd)
	}

	interactive.DeletePending(msg.id, msg.taskHash)

	next, cmd, _ := t.commandPending()
	slog.Debug("pending task dropped", slog.String("label", msg.label))
	return next, cmd
}

func (t TUI) resumePending(msg PendingSelect) (tea.Model, tea.Cmd) {
	if t.running || t.connecting {
		return t, nil
	}
	if _, ok := interactive.LoadPendingInfo(msg.id, msg.taskHash); !ok {
		return t, noPendingNotice()
	}
	return t.startPending(msg.id, msg.taskHash)
}

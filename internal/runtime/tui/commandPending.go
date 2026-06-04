package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

type PendingSelect struct {
	id       string
	taskHash string
}

func (t TUI) commandPending() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}

	hashes := interactive.ListPendingTasks(sid)
	if len(hashes) == 0 {
		return t, tea.Println(hintStyle.Render("no pending tasks") + "\n"), true
	}

	options := make([]string, 0, len(hashes))
	values := make([]string, 0, len(hashes))
	for _, h := range hashes {
		info, ok := interactive.LoadPendingInfo(sid, h)
		if !ok {
			continue
		}
		label := truncate(h, 8)
		if info.Objective != "" {
			label = truncate(info.Objective, 60)
		}
		if info.HasQuestions {
			label += " (awaiting answer)"
		}
		options = append(options, label)
		values = append(values, h)
	}

	if len(options) == 0 {
		return t, tea.Println(hintStyle.Render("no pending tasks") + "\n"), true
	}

	sessionID := sid
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   fmt.Sprintf("Pending tasks (%d)", len(options)),
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return PendingSelect{id: sessionID, taskHash: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) resumePending(msg PendingSelect) (tea.Model, tea.Cmd) {
	info, ok := interactive.LoadPendingInfo(msg.id, msg.taskHash)
	if !ok {
		return t, tea.Println(errorStyle.Render("[!] pending task not found") + "\n")
	}

	if !info.HasQuestions {
		content, err := interactive.LoadResumeMessage(msg.id, msg.taskHash, nil)
		if err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] load resume: %v", err)) + "\n")
		}
		return t.startResume(ResumeExec{SessionID: msg.id, Content: content, PendingTask: msg.taskHash})
	}

	meta, err := interactive.LoadPendingQuestions(msg.id, msg.taskHash)
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] load pending: %v", err)) + "\n")
	}

	sid := msg.id
	taskHash := msg.taskHash
	runtime.AskUser(runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sid,
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: meta},
	}, func(reply runtime.Reply) {
		if reply.Error != nil {
			interactive.CleanupPending(sid, taskHash)
			return
		}
		runtime.TriggerResume(sid, taskHash, reply.Answers)
	})
	return t, nil
}

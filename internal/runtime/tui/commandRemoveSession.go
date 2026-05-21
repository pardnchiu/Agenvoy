package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type RemoveSessionConfirm1 struct {
	id  string
	yes bool
}

type RemoveSessionConfirm2 struct {
	id  string
	yes bool
}

func (t TUI) commandRemoveSession() (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(hintStyle.Render("no active session") + "\n"), true
	}

	label := utils.ShortenSessionID(sid)
	if name, _ := session.GetBot(sid); name != "" && name != sid {
		label = fmt.Sprintf("%s (%s)", name, label)
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   fmt.Sprintf("Remove session %s ?", label),
		options: []string{"No", "Yes"},
		values:  []string{"no", "yes"},
		cursor:  0,
		onConfirm: func(chosen string) any {
			return RemoveSessionConfirm1{id: sid, yes: chosen == "yes"}
		},
	}
	return t, nil, true
}

func (t TUI) openRemoveSessionConfirm2(sid string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "Are you sure? This cannot be undone.",
		subtitle: fmt.Sprintf("History and tool data for %s will be permanently deleted.", utils.ShortenSessionID(sid)),
		options:  []string{"No", "Yes, delete it"},
		values:   []string{"no", "yes"},
		cursor:   0,
		onConfirm: func(chosen string) any {
			return RemoveSessionConfirm2{id: sid, yes: chosen == "yes"}
		},
	}
	return t, nil
}

func (t TUI) runRemoveSession(sid string) (TUI, tea.Cmd) {
	target := pickAlternateSession(sid)
	if target == "" {
		created, err := session.CreateSession("cli-")
		if err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] create fallback session failed: %v", err)) + "\n")
		}
		target = created
	}

	deletedKeys := deleteSessionHistKeys(sid)

	sessionDir := filepath.Join(filesystem.SessionsDir, sid)
	if err := os.RemoveAll(sessionDir); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] remove session dir: %v", err)) + "\n")
	}

	t.currentSessionID = target
	t.currentSessionName, _ = session.GetBot(target)
	t = t.restartTailer()

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0
	t.currentModel = ""
	t.activity = ""

	lines := []string{
		hintStyle.Render(fmt.Sprintf("⎯ removed: %s (%d keys, dir purged)", utils.ShortenSessionID(sid), deletedKeys)),
		hintStyle.Render(fmt.Sprintf("⎯ switched to: %s", utils.ShortenSessionID(target))),
	}

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
	}
	seq = append(seq, loadSessionTail(target)...)
	seq = append(seq, tea.Println(strings.Join(lines, "\n")+"\n"))
	return t, tea.Sequence(seq...)
}

func pickAlternateSession(exclude string) string {
	for _, s := range listSessions() {
		if s.id == exclude {
			continue
		}
		return s.id
	}
	return ""
}

func deleteSessionHistKeys(sid string) int {
	db := torii.DB(torii.DBSessionHist)
	keys := db.Keys(sid + ":*")
	if len(keys) == 0 {
		return 0
	}
	return db.Del(keys...)
}

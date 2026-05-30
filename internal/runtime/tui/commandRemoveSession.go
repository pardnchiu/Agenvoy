package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session"
	sessionBot "github.com/pardnchiu/agenvoy/internal/session/bot"
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
	if name, _ := sessionBot.Get(sid); name != "" && name != sid {
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
	deletedKeys := deleteSessionHistKeys(sid)

	sessionDir := filesystem.SessionDir(sid)
	if err := os.RemoveAll(sessionDir); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] remove session dir: %v", err)) + "\n")
	}

	fallback := pickAlternateSession(sid)
	if fallback == "" {
		created, err := session.CreateSession("cli-")
		if err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] create fallback session failed: %v", err)) + "\n")
		}
		fallback = created
	}

	t.currentSessionID = fallback
	t.currentSessionName, _ = sessionBot.Get(fallback)
	t = t.restartTailer()
	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0
	t.currentModel = ""
	t.activity = ""

	popup := popupSwitch(fallback)
	if popup != nil {
		popup.title = "Removed. Switch to which session?"
		popup.onConfirm = func(chosen string) any {
			if chosen == "" {
				return SessionNew{}
			}
			return SessionSelect{id: chosen}
		}
		t.popup = popup
	}

	lines := []string{
		hintStyle.Render(fmt.Sprintf("⎯ removed: %s (%d keys, dir purged)", utils.ShortenSessionID(sid), deletedKeys)),
	}

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
	}
	seq = append(seq, loadSessionTail(fallback)...)
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

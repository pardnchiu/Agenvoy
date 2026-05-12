package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type Session struct {
	id   string
	name string
}

func (t TUI) commandSwitch(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) >= 2 {
		name := strings.Join(parts[1:], " ")
		id := session.GetSessionIDByName(name)
		if id == "" {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] session %q not found", name)) + "\n"), true
		}
		next, cmd := t.runCommandSwitch(id)
		return next, cmd, true
	}

	popup := popupSwitch(t.currentSessionID)
	if popup == nil {
		return t, tea.Println(hintStyle.Render("no sessions available") + "\n"), true
	}
	popup.onConfirm = func(chosen string) any {
		if chosen == "" {
			return SessionNew{}
		}
		return SessionSelect{id: chosen}
	}
	t.popup = popup
	return t, nil, true
}

func (t TUI) runCommandSwitch(id string) (TUI, tea.Cmd) {
	if id == t.currentSessionID {
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ already on: %s", utils.ShortenSessionID(id))) + "\n")
	}
	if err := changeSession(id); err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] switch failed: %v", err)) + "\n")
	}
	previous := t.currentSessionID
	t.currentSessionID = id
	t.currentSessionName, _ = session.GetBot(id)
	t = t.restartTailer()

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0
	t.currentModel = ""
	t.activity = ""

	switchLines := []string{hintStyle.Render(fmt.Sprintf("⎯ switched to: %s", utils.ShortenSessionID(id)))}
	if previous != "" && previous != id {
		switchLines = append(switchLines, hintStyle.Render(fmt.Sprintf("  previous: %s", utils.ShortenSessionID(previous))))
	}
	switchBlock := tea.Println(strings.Join(switchLines, "\n") + "\n")

	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Println(headerBlock(t.cwd, t.daemonStatus, t.discordStatus)),
	}
	seq = append(seq, loadSessionTail(id)...)
	seq = append(seq, switchBlock)
	return t, tea.Sequence(seq...)
}

func listSessions() []Session {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}

	results := make([]Session, 0, len(dirs))
	for _, dir := range dirs {
		sid := dir.Name
		if !strings.HasPrefix(sid, "cli-") && !strings.HasPrefix(sid, "http-") {
			continue
		}
		name, _ := session.GetBot(sid)
		results = append(results, Session{
			id:   sid,
			name: name,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].id < results[j].id
	})
	return results
}

func popupSwitch(sid string) *Popup {
	sessions := listSessions()

	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].id == sid && sessions[j].id != sid {
			return true
		}
		if sessions[j].id == sid && sessions[i].id != sid {
			return false
		}
		return sessions[i].id < sessions[j].id
	})

	names := make([]string, 0, len(sessions)+1)
	sids := make([]string, 0, len(sessions)+1)
	cursor := 0
	for i, e := range sessions {
		short := utils.ShortenSessionID(e.id)
		label := short
		if e.name != "" && e.name != e.id {
			label = fmt.Sprintf("%s (%s)", e.name, short)
		}
		if e.id == sid {
			label += "  [current]"
			cursor = i
		}
		names = append(names, label)
		sids = append(sids, e.id)
	}

	names = append(names, "(new session)")
	sids = append(sids, "")

	return &Popup{
		kind:    popupSingleSelect,
		title:   "Switch session",
		options: names,
		values:  sids,
		cursor:  cursor,
	}
}

func changeSession(target string) error {
	cfg, err := session.Load()
	if err != nil {
		return fmt.Errorf("session.Load: %w", err)
	}

	cfg.SessionID = target

	if err := session.Save(cfg); err != nil {
		return fmt.Errorf("session.Save: %w", err)
	}
	return nil
}

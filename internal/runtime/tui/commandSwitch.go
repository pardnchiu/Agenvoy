package tui

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionmanager "github.com/pardnchiu/agenvoy/internal/session"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type Session struct {
	id   string
	name string
}

const sessionBotPrefix = "bot:"

func (t TUI) commandSessions(parts []string) (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if len(parts) == 2 && slices.Contains([]string{"name", "id", "role"}, parts[1]) {
		if sid == "" {
			return t, notice(msgError("no current session") + "\n"), true
		}
		next, cmd := t.openBotField(sid, parts[1])
		return next, cmd, true
	}
	if len(parts) >= 2 {
		selfID := strings.Join(parts[1:], " ")
		id := sessionmanager.GetSessionIDBySelfID(selfID)
		if id == "" {
			return t, notice(msgError(fmt.Sprintf("no session with self id %q", selfID)) + "\n"), true
		}
		next, cmd := t.runCommandSwitch(id)
		return next, cmd, true
	}

	popup := popupSwitch(t.currentSessionID)
	if popup == nil {
		slog.Debug("tui: no sessions available")
		return t, nil, true
	}
	popup.subtitle = "pick a session to switch to"
	if sid != "" {
		popup.subtitle = "pick a session to switch to  or edit the current session's name / self id / role below"
	}
	fillSessions := popup.onTab
	popup.onTab = func(p *Popup) tea.Cmd {
		fillSessions(p)
		if sid == "" {
			return nil
		}
		selfID, name, _ := configbot.GetPersona(sid)
		withValue := func(desc, value string) string {
			if value == "" {
				return hintStyle.Render(desc)
			}
			return hintStyle.Render(desc+" ") + okayStyle.Render("["+value+"]")
		}
		p.options = append(p.options, "")
		p.values = append(p.values, "")
		p.options = append(p.options, optionColumn([]string{"name", "self id", "role"}, []string{
			withValue("session display name", name),
			withValue("used to call this session by id", selfID),
			hintStyle.Render("system prompt for this session"),
		})...)
		p.values = append(p.values, sessionBotPrefix+"name", sessionBotPrefix+"id", sessionBotPrefix+"role")
		return nil
	}
	popup.onTab(popup)
	popup.onConfirm = func(chosen string) any {
		if field, ok := strings.CutPrefix(chosen, sessionBotPrefix); ok {
			return BotFieldPick{field: field}
		}
		return SessionSelect{id: chosen}
	}
	popup.onDelete = func(chosen string) any {
		if strings.HasPrefix(chosen, sessionBotPrefix) {
			return nil
		}
		return SessionDeletePick{id: chosen}
	}
	t.popup = popup
	return t, nil, true
}

func (t TUI) runCommandSwitch(id string) (TUI, tea.Cmd) {
	if id == t.currentSessionID {
		return t, nil
	}
	previous := t.currentSessionID
	t.currentSessionID = id
	t.currentSessionName, _ = configbot.Get(id)
	t.inputHistory = loadInputHistory(id)
	t.inputHistoryIdx = -1
	t = t.restartTailer()

	t.tokens = 0
	t.lastIn = 0
	t.lastContext = 0
	t.lastOut = 0
	t.lastCacheRead = 0
	t.lastCacheCreate = 0
	t.currentModel = ""
	t.activity = ""

	slog.Debug("session switched", slog.String("to", id), slog.String("from", previous))

	return t, tea.Sequence(
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.lineStatus, t.currentSessionID)),
	)
}

func listSessions() []Session {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return nil
	}

	results := make([]Session, 0, len(dirs))
	for _, dir := range dirs {
		sid := dir.Name
		if strings.HasPrefix(sid, "temp-") || strings.HasPrefix(sid, ".") {
			continue
		}
		refreshBotName(sid)
		name, _ := configbot.Get(sid)
		results = append(results, Session{
			id:   sid,
			name: name,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if a, b := sessionRank(results[i].id), sessionRank(results[j].id); a != b {
			return a < b
		}
		return results[i].id < results[j].id
	})
	return results
}

func sessionRank(sessionID string) int {
	switch sessionPrefix(sessionID) {
	case "cli-":
		return 0
	case "tg-":
		return 1
	case "dc-":
		return 2
	case "ln-":
		return 3
	case "chat-":
		return 4
	case "temp-":
		return 5
	default:
		return 3
	}
}

func sessionPrefix(sessionID string) string {
	head, _, ok := strings.Cut(sessionID, "-")
	if !ok {
		return sessionID
	}
	return head + "-"
}

func sessionTabs(sessions []Session) []string {
	seen := map[string]bool{}
	prefixes := make([]string, 0, len(sessions))
	for _, e := range sessions {
		prefix := sessionPrefix(e.id)
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		prefixes = append(prefixes, prefix)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		if a, b := sessionRank(prefixes[i]), sessionRank(prefixes[j]); a != b {
			return a < b
		}
		return prefixes[i] < prefixes[j]
	})
	if len(prefixes) < 2 {
		return nil
	}
	return append([]string{"all"}, prefixes...)
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
		return false
	})

	popup := &Popup{
		kind:        popupSingleSelect,
		title:       "/session",
		maxVisible:  cmdSelectorMaxVisible,
		enterAction: "switch",
		tabs:        sessionTabs(sessions),
	}
	popup.onTab = func(p *Popup) tea.Cmd {
		fillSwitchOptions(p, sessions, sid)
		return nil
	}
	popup.onTab(popup)
	return popup
}

func fillSwitchOptions(p *Popup, sessions []Session, sid string) {
	tab := ""
	if p.tabIdx > 0 && p.tabIdx < len(p.tabs) {
		tab = p.tabs[p.tabIdx]
	}

	list := make([]Session, 0, len(sessions))
	for _, e := range sessions {
		if tab == "" || strings.HasPrefix(e.id, tab) {
			list = append(list, e)
		}
	}

	shorts := make([]string, len(list))
	sidMax := 0
	for i, e := range list {
		shorts[i] = utils.ShortenSessionID(e.id)
		if n := len(shorts[i]); n > sidMax {
			sidMax = n
		}
	}

	names := make([]string, 0, len(list)+1)
	sids := make([]string, 0, len(list)+1)
	cursor := 0
	for i, e := range list {
		padded := shorts[i]
		if len(padded) < sidMax {
			padded += strings.Repeat(" ", sidMax-len(padded))
		}
		label := padded
		if e.name != "" && e.name != e.id {
			label += "  (" + e.name + ")"
		}
		if e.id == sid {
			label += " " + systemStyle.Render("[current]")
			cursor = i
		}
		names = append(names, label)
		sids = append(sids, e.id)
	}

	p.options = names
	p.values = sids
	p.cursor = cursor
}

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/session"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

type BotNameSubmit struct {
	name string
}

type BotBodySubmit struct {
	name string
	body string
}

type BotSaved struct {
	name string
	err  error
}

func (t TUI) commandBot(parts []string) (TUI, tea.Cmd, bool) {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return t, tea.Println(errorStyle.Render("[!] no current session") + "\n"), true
	}

	if len(parts) >= 3 {
		name := strings.TrimSpace(parts[1])
		body := strings.TrimSpace(strings.Join(parts[2:], " "))
		if cmd, ok := t.botCheckConflict(sid, name); !ok {
			return t, cmd, true
		}
		return t, t.botSaveCmd(sid, name, body), true
	}

	refreshBotName(sid)
	existingName, existingBody := configBot.Get(sid)
	t.popup = &Popup{
		kind:  popupText,
		title: "Bot name",
		input: existingName,
		onConfirm: func(value string) any {
			return BotNameSubmit{name: strings.TrimSpace(value)}
		},
	}
	t.botBodyDraft = existingBody
	return t, nil, true
}

func (t TUI) botCheckConflict(sid, name string) (tea.Cmd, bool) {
	if name == "" {
		return tea.Println(errorStyle.Render("[!] bot name required") + "\n"), false
	}
	if owner := session.GetSessionID(name); owner != "" && owner != sid {
		return tea.Println(errorStyle.Render(fmt.Sprintf("[!] bot name %q already used by session %s", name, owner)) + "\n"), false
	}
	return nil, true
}

func (t TUI) openBotBodyPopup(name string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		title:     fmt.Sprintf("Bot description (%s)", name),
		multiline: true,
		input:     t.botBodyDraft,
		onConfirm: func(value string) any {
			return BotBodySubmit{name: name, body: value}
		},
	}
	t.botBodyDraft = ""
	return t, nil
}

func (t TUI) botSaveCmd(sid, name, body string) tea.Cmd {
	err := configBot.Save(sid, name, body, true)
	return func() tea.Msg { return BotSaved{name: name, err: err} }
}

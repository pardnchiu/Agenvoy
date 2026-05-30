package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type AdminChannelSubmit struct {
	value string
}

func adminChannelLabel(prefix string, e utils.ChatEntry) string {
	if strings.TrimSpace(e.Name) == "" {
		return prefix + " · " + e.ID
	}
	return prefix + " · " + e.Name + " (" + e.ID + ")"
}

func (t TUI) commandAdminChannel(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		value := strings.TrimSpace(strings.Join(parts[1:], " "))
		return t, func() tea.Msg { return AdminChannelSubmit{value: value} }, true
	}

	current := ""
	if cfg, err := session.Load(); err == nil && cfg != nil {
		current = strings.TrimSpace(cfg.AdminChannel)
	}

	var options, values []string
	add := func(label, value string) {
		if value != "" && value == current {
			label = "✓ " + label
		}
		options = append(options, label)
		values = append(values, value)
	}

	add("(clear) · code stays log-only", "")
	for _, e := range utils.ListChats(filesystem.TelegramAuthPath) {
		add(adminChannelLabel("tg", e), "tg@"+e.ID)
	}
	for _, e := range utils.ListChats(filesystem.DiscordAuthPath) {
		add(adminChannelLabel("dc", e), "dc@"+e.ID)
	}

	cursor := 0
	for i, v := range values {
		if v != "" && v == current {
			cursor = i
			break
		}
	}

	t.popup = &Popup{
		kind:       popupSingleSelect,
		title:      "Admin Channel · relay new-chat verification codes",
		subtitle:   "pick an authorized chat/channel · only listed (already-verified) targets receive codes",
		options:    options,
		values:     values,
		cursor:     cursor,
		maxVisible: cmdSelectorMaxVisible,
		onConfirm: func(chosen string) any {
			return AdminChannelSubmit{value: chosen}
		},
	}
	return t, nil, true
}

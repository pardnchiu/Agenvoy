package tui

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

type ReplyLanguageSelect struct {
	code string
}

func (t TUI) commandReplyLanguage() (TUI, tea.Cmd, bool) {
	languages := filesystem.ReplyLangOptions()
	if len(languages) == 0 {
		slog.Debug("tui: no languages available")
		return t, nil, true
	}

	keys := make([]string, 0, len(languages))
	details := make([]string, 0, len(languages))
	values := make([]string, 0, len(languages))
	cursor := 0
	for i, one := range languages {
		detail := one.Label
		if one.Code == filesystem.ConfigReplyLang {
			detail += "  " + systemStyle.Render("[current]")
			cursor = i
		}
		keys = append(keys, "["+one.Code+"]")
		details = append(details, detail)
		values = append(values, one.Code)
	}
	options := optionColumn(keys, details)

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Select reply language",
		options: options,
		values:  values,
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return ReplyLanguageSelect{code: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) runReplyLanguageSelect(code string) (TUI, tea.Cmd) {
	lang := filesystem.CanonicalReplyLang(code)

	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("reply-language: %v", err)) + "\n")
	}
	dic["reply_lang"] = lang
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("reply-language: %v", err)) + "\n")
	}
	filesystem.ConfigReplyLang = lang

	slog.Debug("reply language updated", slog.String("lang", lang))
	return t.openConfig(configReplyLang), nil
}

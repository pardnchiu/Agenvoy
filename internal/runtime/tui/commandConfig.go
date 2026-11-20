package tui

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/startup"
)

const (
	configStartup   = "startup"
	configReplyLang = "reply_lang"
	configOutputDir = "output_dir"
	configAdminChat = "admin_chat"
	configOfficial  = "official_guide"
)

type ConfigSelect struct {
	key string
}

type StartupDone struct {
	action string
	detail string
	err    error
}

func (t TUI) commandConfig() (TUI, tea.Cmd, bool) {
	return t.openConfig(""), nil, true
}

func (t TUI) openConfig(focus string) TUI {
	startupValue := hintStyle.Render("disable")
	if startup.State() {
		startupValue = okayStyle.Render("enable")
	}
	officialValue := okayStyle.Render("enable")
	if cfg, err := config.Load(); err == nil && cfg.OfficialGuideOff {
		officialValue = hintStyle.Render("disable")
	}
	names := []string{"Startup on login", "Reply language", "Output dir", "Admin Channel", "Official guide"}
	settings := []string{
		startupValue,
		whiteStyle.Render(filesystem.CanonicalReplyLang(filesystem.GetReplyLang())),
		whiteStyle.Render(filesystem.OutputDir()),
		adminChatValue(),
		officialValue,
	}
	values := []string{configStartup, configReplyLang, configOutputDir, configAdminChat, configOfficial}
	options := optionColumn(names, settings)

	input := newPopupInput("", false)
	input.Placeholder = "Search settings..."
	input.SetPromptFunc(2, func(int) string {
		return hintStyle.Render("/ ")
	})

	t.popup = &Popup{
		kind:        popupSingleSelect,
		title:       "/config",
		options:     options,
		values:      values,
		allOptions:  options,
		allValues:   values,
		searchable:  true,
		input:       input,
		cursor:      max(slices.Index(values, focus), 0),
		enterAction: "change",
		onConfirm: func(chosen string) any {
			return ConfigSelect{key: chosen}
		},
	}
	return t
}

func (t TUI) runConfigSelect(key string) (TUI, tea.Cmd) {
	switch key {
	case configStartup:
		if startup.State() {
			return t, setStartup("disable")
		}
		return t, setStartup("enable")

	case configReplyLang:
		next, cmd, _ := t.commandReplyLanguage()
		return next, cmd

	case configOutputDir:
		next, cmd, _ := t.commandOutputDir()
		return next, cmd

	case configAdminChat:
		next, cmd, _ := t.commandAdminChannel(nil)
		return next, cmd

	case configOfficial:
		return t.toggleOfficialGuide()
	}
	return t, nil
}

func setStartup(action string) tea.Cmd {
	return func() tea.Msg {
		var (
			detail string
			err    error
		)
		if action == "enable" {
			detail, err = startup.Enable()
		} else {
			detail, err = startup.Disable()
		}
		return StartupDone{action: action, detail: detail, err: err}
	}
}

func (t TUI) toggleOfficialGuide() (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("official-guide: %v", err)) + "\n")
	}
	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("official-guide: %v", err)) + "\n")
	}
	dic["official_guide_disabled"] = !cfg.OfficialGuideOff
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("official-guide: %v", err)) + "\n")
	}
	return t.openConfig(configOfficial), nil
}

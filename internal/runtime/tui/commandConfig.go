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
	configCmdNet    = "command_network"
	configHookNet   = "hook_network"
	configLAN       = "lan_access"
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
	cmdNetValue := okayStyle.Render("enable")
	hookNetValue := okayStyle.Render("enable")
	lanValue := hintStyle.Render("disable")
	if cfg, err := config.Load(); err == nil {
		if cfg.OfficialGuideOff {
			officialValue = hintStyle.Render("disable")
		}
		if cfg.CommandNetOff {
			cmdNetValue = hintStyle.Render("disable")
		}
		if cfg.HookNetOff {
			hookNetValue = hintStyle.Render("disable")
		}
		if cfg.LANAccess {
			lanValue = warnStyle.Render("enable  do not leave it on")
		}
	}

	names := []string{"Startup on login", "Reply / WebUI language", "Output dir", "Admin Channel", "Official guide", "Command network", "Hook network", "LAN access"}
	settings := []string{
		startupValue,
		whiteStyle.Render(filesystem.CanonicalReplyLang(filesystem.GetReplyLang())),
		whiteStyle.Render(filesystem.OutputDir()),
		adminChatValue(),
		officialValue,
		cmdNetValue,
		hookNetValue,
		lanValue,
	}
	values := []string{configStartup, configReplyLang, configOutputDir, configAdminChat, configOfficial, configCmdNet, configHookNet, configLAN}
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

	case configCmdNet:
		return t.toggleCommandNetwork()

	case configHookNet:
		return t.toggleHookNetwork()

	case configLAN:
		return t.openLANAccess()
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

func (t TUI) toggleCommandNetwork() (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("command-network: %v", err)) + "\n")
	}
	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("command-network: %v", err)) + "\n")
	}
	dic["command_network_disabled"] = !cfg.CommandNetOff
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("command-network: %v", err)) + "\n")
	}
	return t.openConfig(configCmdNet), nil
}

func (t TUI) toggleHookNetwork() (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("hook-network: %v", err)) + "\n")
	}
	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("hook-network: %v", err)) + "\n")
	}
	dic["hook_network_disabled"] = !cfg.HookNetOff
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("hook-network: %v", err)) + "\n")
	}
	return t.openConfig(configHookNet), nil
}

type ConfigLANChoice struct {
	choice string
}

func (t TUI) openLANAccess() (TUI, tea.Cmd) {
	cfg, err := config.Load()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("lan-access: %v", err)) + "\n")
	}
	if cfg.LANAccess {
		return t.setLANAccess(false)
	}
	t.popup = &Popup{
		kind:     popupSingleSelect,
		title:    "LAN access lets any device on your network use the API without a password",
		subtitle: "for short use only  do not leave it on  turn it off when done",
		options:  []string{"Cancel", "Enable"},
		values:   []string{"cancel", "enable"},
		onConfirm: func(chosen string) any {
			return ConfigLANChoice{choice: chosen}
		},
	}
	return t, nil
}

func (t TUI) runLANChoice(choice string) (TUI, tea.Cmd) {
	if choice == "enable" {
		return t.setLANAccess(true)
	}
	return t.openConfig(configLAN), nil
}

func (t TUI) setLANAccess(enable bool) (TUI, tea.Cmd) {
	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("lan-access: %v", err)) + "\n")
	}
	dic["lan_access_enabled"] = enable
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("lan-access: %v", err)) + "\n")
	}
	if enable {
		return t.openConfig(configLAN), notice(msgWarn("LAN access enabled: any device on your network can use the API without a password; turn it off when done") + "\n")
	}
	return t.openConfig(configLAN), nil
}

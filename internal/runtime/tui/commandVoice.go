package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/session/config"
)

func (t TUI) commandVoice(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "enable":
			if voiceNeedsGeminiKey() {
				next, cmd := t.openVoiceKeyPrompt()
				return next, cmd, true
			}
			return t, setVoice("enable"), true
		case "disable":
			return t, setVoice("disable"), true
		}
	}

	enabled := false
	if cfg, err := config.Load(); err == nil && cfg != nil {
		enabled = cfg.EnableVoice
	}
	cursor := 0
	if enabled {
		cursor = 1
	}
	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Voice",
		options: []string{"enable", "disable"},
		values:  []string{"enable", "disable"},
		cursor:  cursor,
		onConfirm: func(chosen string) any {
			return VoiceAction{action: chosen}
		},
	}
	return t, nil, true
}

type VoiceAction struct {
	action string
}

type VoiceDone struct {
	action string
	err    error
}

type VoiceKeySubmit struct {
	token string
}

func voiceNeedsGeminiKey() bool {
	return strings.TrimSpace(keychain.Get("GEMINI_API_KEY")) == ""
}

func (t TUI) openVoiceKeyPrompt() (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:     popupText,
		title:    "Voice · GEMINI_API_KEY",
		subtitle: "required for voice transcription / voice reply · Enter to submit · Esc to cancel",
		onConfirm: func(value string) any {
			return VoiceKeySubmit{token: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func setVoice(action string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return VoiceDone{action: action, err: fmt.Errorf("session.Load: %w", err)}
		}
		cfg.EnableVoice = action == "enable"
		if err := config.Save(cfg); err != nil {
			return VoiceDone{action: action, err: fmt.Errorf("session.Save: %w", err)}
		}
		return VoiceDone{action: action}
	}
}

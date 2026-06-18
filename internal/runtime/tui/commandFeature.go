package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type FeatureSelect struct {
	feature string
}

func (t TUI) commandFeature(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "voice":
			return t.commandVoice(parts[1:])
		case "image2":
			return t.commandImage2(parts[1:])
		case "kuradb":
			return t.commandKuradb(parts[1:])
		}
	}

	t.popup = &Popup{
		kind: popupSingleSelect,
		title: "Feature",
		options: []string{
			"voice   enable / disable voice message",
			"image2  enable / disable image generation",
			"kuradb  enable / disable KuraDB RAG",
		},
		values: []string{"voice", "image2", "kuradb"},
		onConfirm: func(chosen string) any {
			return FeatureSelect{feature: chosen}
		},
	}
	return t, nil, true
}

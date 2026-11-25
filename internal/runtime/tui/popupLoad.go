package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type PopupLoaded struct {
	target *Popup
	popup  *Popup
}

func (t TUI) openLoading(title string, build func() *Popup) (TUI, tea.Cmd) {
	target := &Popup{
		kind:        popupSingleSelect,
		title:       title,
		styledLines: []string{hintStyle.Render("  loading...")},
		readOnly:    true,
	}
	t.popup = target
	return t, func() tea.Msg {
		return PopupLoaded{target: target, popup: build()}
	}
}

func (t TUI) runPopupLoaded(msg PopupLoaded) (TUI, tea.Cmd) {
	if t.popup != msg.target || msg.popup == nil {
		return t, nil
	}
	if msg.popup.back == nil {
		msg.popup.back = msg.target.back
	}
	t.popup = msg.popup
	return t, nil
}

func messagePopup(title, text string) *Popup {
	return &Popup{
		kind:        popupSingleSelect,
		title:       title,
		styledLines: []string{hintStyle.Render("  " + text)},
		readOnly:    true,
	}
}

func errorPopup(title, text string) *Popup {
	return &Popup{
		kind:        popupSingleSelect,
		title:       title,
		styledLines: []string{errorStyle.Render("  " + text)},
		readOnly:    true,
	}
}

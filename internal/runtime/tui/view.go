package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func (t TUI) View() string {
	if t.quitting {
		return ""
	}
	if t.popup != nil {
		return t.viewPopup()
	}
	return t.viewIdle()
}

func (t TUI) viewIdle() string {
	width := t.width
	if width < 20 {
		width = 80
	}

	if t.onceCall {
		if t.awaitingExit {
			return ""
		}
		if t.running {
			return "\n" + t.viewThinking() + "\n"
		}
		return ""
	}

	left := hintStyle.Render(" / commands · enter send · alt+enter newline · esc cancel")
	right := t.sessionTag()

	if t.mode == webMode {
		left = hintStyle.Render(" / commands · enter send · alt+enter newline · /mode to switch")
	}

	prefix := "\n"
	var top string
	if t.running {
		prefix = ""
		top = t.viewThinking() + "\n\n"
	}

	if t.selector != nil {
		top += renderCmdSelector(t.selector) + "\n"
	}

	box := textAreaStyle.Width(width - 2).Render(t.textarea.View())

	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	pad = max(pad, 1)
	return prefix + top + box + "\n" + left + strings.Repeat(" ", pad) + right
}

func (t TUI) viewThinking() string {
	verb := activityVerb(t.activity)
	elapsed := formatTime(int(time.Since(t.runStartedAt).Seconds()))

	detail := []string{elapsed}
	if t.currentModel != "" {
		detail = append(detail, t.currentModel)
	}
	detail = append(detail, "esc to interrupt")

	return systemStyle.Render(t.spinner.View()) + " " +
		systemStyle.Render(verb+"…") + " " +
		hintStyle.Render("("+strings.Join(detail, " · ")+")")
}

func (t TUI) viewPopup() string {
	width := t.width
	if width < 20 {
		width = 80
	}
	p := t.popup
	if p == nil {
		return ""
	}

	body := []string{systemStyle.Bold(true).Render("⏺ " + p.title)}
	if p.subtitle != "" {
		body = append(body, hintStyle.Render(p.subtitle))
		body = append(body, "")
	} else {
		body = append(body, "")
	}

	switch p.kind {
	case popupConfirm, popupSingleSelect:
		total := len(p.options)
		visible := p.maxVisible
		if visible <= 0 && p.kind == popupSingleSelect {
			visible = cmdSelectorMaxVisible
		}
		start, end := 0, total
		windowed := visible > 0 && total > visible
		if windowed {
			start, end = windowRange(p.cursor, total, visible)
		}
		for i := start; i < end; i++ {
			opt := p.options[i]
			marker := "  "
			line := opt
			if i == p.cursor {
				marker = systemStyle.Render("> ")
				line = systemStyle.Render(opt)
			}
			body = append(body, marker+line)
		}
		if windowed {
			body = append(body, hintStyle.Render(fmt.Sprintf("  %d/%d", p.cursor+1, total)))
		}
		body = append(body, "")
		body = append(body, hintStyle.Render("↑/↓ select · enter confirm · esc cancel"))

	case popupMultiSelect:
		total := len(p.options)
		visible := p.maxVisible
		if visible <= 0 {
			visible = cmdSelectorMaxVisible
		}
		start, end := 0, total
		windowed := total > visible
		if windowed {
			start, end = windowRange(p.cursor, total, visible)
		}
		for i := start; i < end; i++ {
			opt := p.options[i]
			cursor := "  "
			if i == p.cursor {
				cursor = systemStyle.Render("> ")
			}
			check := "[ ]"
			if p.multi[i] {
				check = systemStyle.Render("[x]")
			}
			body = append(body, fmt.Sprintf("%s%s %s", cursor, check, opt))
		}
		if windowed {
			body = append(body, hintStyle.Render(fmt.Sprintf("  %d/%d", p.cursor+1, total)))
		}
		body = append(body, "")
		body = append(body, hintStyle.Render("↑/↓ move · space toggle · enter confirm · esc cancel"))

	case popupText:
		if p.multiline {
			lines := strings.Split(p.input, "\n")
			for i, ln := range lines {
				prefix := systemStyle.Render("  ")
				if i == 0 {
					prefix = systemStyle.Render("> ")
				}
				if i == len(lines)-1 {
					body = append(body, prefix+ln+systemStyle.Render("▏"))
				} else {
					body = append(body, prefix+ln)
				}
			}
			body = append(body, "")
			body = append(body, hintStyle.Render("ctrl+s confirm · enter newline · esc cancel"))
		} else {
			field := systemStyle.Render("> ") + p.input + systemStyle.Render("▏")
			body = append(body, field)
			body = append(body, "")
			body = append(body, hintStyle.Render("enter confirm · esc cancel"))
		}

	case popupSecret:
		mask := strings.Repeat("•", len([]rune(p.input)))
		field := systemStyle.Render("> ") + mask + systemStyle.Render("▏")
		body = append(body, field)
		body = append(body, "")
		body = append(body, hintStyle.Render("enter confirm · esc cancel · (input hidden)"))

	case popupOAuth:
		if p.oauth != nil {
			if p.oauth.url != "" {
				body = append(body, hintStyle.Render("url:  ")+textStyle.Render(p.oauth.url))
			}
			if p.oauth.userCode != "" {
				body = append(body, hintStyle.Render("code: ")+systemStyle.Render(p.oauth.userCode))
			}
		}
		body = append(body, "")
		body = append(body, hintStyle.Render("enter re-open browser · esc cancel"))
	}

	if len(p.questions) > 1 {
		footer := hintStyle.Render(fmt.Sprintf("question %d/%d", p.questionIdx+1, len(p.questions)))
		body = append(body, footer)
	}

	return popupStyle.Width(width - 4).Render(strings.Join(body, "\n"))
}

func (t TUI) sessionTag() string {
	modeTag := lipgloss.NewStyle().Foreground(t.mode.color()).Render(t.mode.String())
	parts := []string{modeTag}
	if name := t.sessionName(); name != "" {
		parts = append(parts, hintStyle.Render(name))
	}
	return strings.Join(parts, hintStyle.Render(" · ")) + hintStyle.Render("  ")
}

func (t TUI) sessionName() string {
	sid := strings.TrimSpace(t.currentSessionID)
	name := strings.TrimSpace(t.currentSessionName)
	if sid == "" {
		return hintStyle.Render("(no session)")
	}

	short := utils.ShortenSessionID(sid)
	base := short
	if name != "" && name != sid {
		base = fmt.Sprintf("%s (%s)", name, short)
	}

	s := sessionManager.ReadStatus(sid)
	model := strings.TrimSpace(s.Model)
	reasoning := strings.TrimSpace(s.Reasoning)
	if model == "" {
		model = sessionManager.StatusModel
	}
	if reasoning == "" {
		reasoning = sessionManager.StatusReasoning
	}
	return fmt.Sprintf("%s (%s/%s)", base, model, reasoning)
}

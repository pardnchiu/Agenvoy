package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
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

	var confirmMode string
	if t.allowAll {
		confirmMode = errorStyle.Render(" [auto]") + hintStyle.Render(" "+t.shortCwd())
	} else {
		confirmMode = okayStyle.Render(" [safe]") + hintStyle.Render(" "+t.shortCwd())
	}
	right := t.sessionTag()

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

	pad := width - lipgloss.Width(confirmMode) - lipgloss.Width(right)
	pad = max(pad, 1)
	return prefix + top + box + "\n" + confirmMode + strings.Repeat(" ", pad) + right
}

func (t TUI) viewThinking() string {
	var sb strings.Builder
	for _, line := range t.toolBuf {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	verb := activityVerb(t.activity)
	elapsed := formatTime(int(time.Since(t.runStartedAt).Seconds()))

	detail := []string{elapsed}
	if t.currentModel != "" {
		detail = append(detail, t.currentModel)
	}
	detail = append(detail, "esc to interrupt")

	sb.WriteString(systemStyle.Render(t.spinner.View()))
	sb.WriteString(" ")
	sb.WriteString(systemStyle.Render(verb + "…"))
	sb.WriteString(" ")
	sb.WriteString(hintStyle.Render("(" + strings.Join(detail, " · ") + ")"))
	return sb.String()
}

func (t TUI) shortCwd() string {
	cwd := t.cwd
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		switch {
		case cwd == home:
			return "~"
		case strings.HasPrefix(cwd, home+"/"):
			return "~" + cwd[len(home):]
		}
	}
	return cwd
}

func splitOptStyle(s string) (head, tail string) {
	if idx := strings.Index(s, " · "); idx >= 0 {
		return s[:idx], s[idx:]
	}
	return s, ""
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

	body := []string{whiteStyle.Render("⏺ " + p.title)}
	if p.subtitle != "" {
		body = append(body, textStyle.Render(p.subtitle))
	}
	body = append(body, p.styledLines...)
	for _, dl := range p.diffLines {
		if strings.HasPrefix(dl, "- ") {
			body = append(body, diffOldStyle.Render("  "+dl))
		} else {
			body = append(body, diffNewStyle.Render("  "+dl))
		}
	}
	body = append(body, "")

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
		maxLine := max(width-10, 20)
		for i := start; i < end; i++ {
			opt := go_pkg_utils.TruncateString(p.options[i], maxLine)
			marker := "  "
			var line string
			if i == p.cursor {
				marker = systemStyle.Render("> ")
				head, tail := splitOptStyle(opt)
				line = systemStyle.Render(head)
				if tail != "" {
					line += hintStyle.Render(tail)
				}
			} else {
				line = hintStyle.Render(opt)
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
		maxLine := max(width-14, 20)
		for i := start; i < end; i++ {
			opt := go_pkg_utils.TruncateString(p.options[i], maxLine)
			cursor := "  "
			head, tail := splitOptStyle(opt)
			var line string
			if i == p.cursor {
				cursor = systemStyle.Render("> ")
				line = systemStyle.Render(head)
			} else {
				line = whiteStyle.Render(head)
			}
			if tail != "" {
				line += hintStyle.Render(tail)
			}
			check := "[ ]"
			if p.multi[i] {
				check = systemStyle.Render("[x]")
			}
			body = append(body, fmt.Sprintf("%s%s %s", cursor, check, line))
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
	var parts []string
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

	model, reasoning := configBot.GetModel(sid)
	modelPart := hintStyle.Render(model)
	if model != configBot.DefaultModel {
		modelPart = warnStyle.Render(model)
	}
	var reasonPart string
	switch reasoning {
	case "low":
		reasonPart = okayStyle.Render(reasoning)
	case "high":
		reasonPart = errorStyle.Render(reasoning)
	default:
		reasonPart = hintStyle.Render(reasoning)
	}
	return base + hintStyle.Render(" (") + modelPart + hintStyle.Render("/") + reasonPart + hintStyle.Render(")")
}

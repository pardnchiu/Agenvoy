package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/guide"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func (t TUI) View() string {
	if t.quitting {
		return ""
	}
	if t.popup != nil && t.popupOnScreen {
		return t.viewPopup()
	}
	return t.viewIdle()
}

func (t TUI) viewIdle() string {
	width := t.width
	if width < 20 {
		width = 80
	}

	var fastMode string
	if fast.IsEnabled() {
		fastMode = systemStyle.Render("[fast]")
	}

	var guideMode string
	if guide.IsEnabled() {
		if name := guide.Name(t.cwd); name != "" {
			guideMode = warnStyle.Render("[" + name + "]")
		}
	}
	cwd := hintStyle.Render(" " + t.shortCwd())

	var confirmMode string
	if t.allowAll {
		confirmMode = errorStyle.Render("[auto]") + guideMode + cwd
	} else {
		confirmMode = okayStyle.Render("[safe]") + guideMode + cwd
	}

	var top string
	if t.running {
		if block := t.viewThinking(); block != "" {
			top = block + "\n"
		}
	}

	if t.selector != nil {
		top += renderCmdSelector(t.selector) + "\n"
	}

	if t.notice != "" {
		top += noticeBlock(t.notice, t.noticeOffset, width-4) + "\n"
	}

	boxStyle, lineStyle := textAreaStyle, thinkStyle
	if strings.HasPrefix(t.textarea.Value(), "!") {
		boxStyle, lineStyle = textAreaStyle.BorderForeground(colError), errorStyle
		t.textarea.FocusedStyle.Text = errorStyle
		t.textarea.Focus()
		t.textarea.SetPromptFunc(2, func(lineIdx int) string {
			if lineIdx == 0 {
				return errorStyle.Render("$ ")
			}
			return "  "
		})
	}
	box := boxStyle.Width(width - 1).Render(t.textarea.View())
	boxWidth := lipgloss.Width(box)
	bottom := lineStyle.Render(strings.Repeat("─", boxWidth))
	if model := t.modelTag(); model != "" {
		tag := " " + model + " "
		if fill := boxWidth - lipgloss.Width(tag) - 1; fill >= 1 {
			bottom = lineStyle.Render(strings.Repeat("─", fill)) + tag + lineStyle.Render("─")
		}
	}

	if t.connecting {
		return top + box + "\n" + bottom + "\n " + t.spinner.View() + systemStyle.Render(" Connecting")
	}

	status := " " + fastMode + confirmMode
	var right string
	if n := pendingCount(t.currentSessionID); n > 0 {
		right = errorStyle.Render(fmt.Sprintf("pending (%d)", n)) + " "
	}
	if t.quotaText != "" {
		right = hintStyle.Render(t.quotaModel+" ") + renderQuotaBadge(t.quotaText) + " " + right
	}
	if right != "" {
		if gap := boxWidth - lipgloss.Width(status) - lipgloss.Width(right); gap >= 1 {
			status += strings.Repeat(" ", gap) + right
		}
	}

	return top + box + "\n" + bottom + "\n" + status
}

func (t TUI) viewThinking() string {
	var sb strings.Builder
	for _, line := range t.toolBuf {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	for _, name := range t.subOrder {
		block, ok := t.subBuf[name]
		if !ok {
			continue
		}
		sb.WriteString(block.render(name, t.width))
		sb.WriteByte('\n')
	}

	for _, line := range t.toolLog {
		sb.WriteString(hintStyle.Render(go_pkg_utils.TruncateString("  "+line, max(t.width-4, 32))))
		sb.WriteByte('\n')
	}

	verb := activityVerb(t.activity)
	elapsed := formatTime(int(time.Since(t.runStartedAt).Seconds()))

	detail := []string{
		elapsed,
		"esc to interrupt",
	}

	sb.WriteString(systemStyle.Render(t.spinner.View()))
	sb.WriteString(" ")
	sb.WriteString(systemStyle.Render(verb + "..."))
	sb.WriteString(" ")
	sb.WriteString(hintStyle.Render("(" + strings.Join(detail, "  ") + ")"))

	if block := renderTodoList(t.todos, t.spinner.View()); block != "" {
		sb.WriteString("\n\n")
		sb.WriteString(block)
	}
	if block := renderWaitBlock(t.pendingSteer); block != "" {
		sb.WriteString("\n\n")
		sb.WriteString(block)
	}
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
	trimmed := strings.TrimLeft(s, " ")
	lead := len(s) - len(trimmed)
	if idx := strings.Index(trimmed, "  "); idx >= 0 {
		return s[:lead+idx], s[lead+idx:]
	}
	return s, ""
}

func renderPopupTabs(p *Popup) string {
	cells := make([]string, len(p.tabs))
	for i, tab := range p.tabs {
		label := strings.TrimSuffix(tab, "-")
		if i == p.tabIdx {
			cells[i] = systemStyle.Render("● " + label)
			continue
		}
		cells[i] = hintStyle.Render("○ " + label)
	}
	return strings.Join(cells, "   ")
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

	header := ""
	switch {
	case len(p.tabs) > 1:
		header = renderPopupTabs(p)
	case len(p.restricted) > 0:
		header = errorStyle.Render("⚠ " + p.title)
	case p.title != "":
		header = systemStyle.Render("● " + strings.TrimPrefix(p.title, "/"))
	}
	divider := thinkStyle.Render(strings.Repeat("─", width))
	headerHeight := 0
	if header != "" {
		header = popupStyle.Width(width).Render(header) + "\n" + divider
	}
	if p.searchable && (p.kind == popupConfirm || p.kind == popupSingleSelect || p.kind == popupMultiSelect) {
		p.input.SetWidth(max(width-4, 20))
		if header != "" {
			header += "\n"
		}
		header += popupStyle.Width(width).Render(p.input.View()) + "\n" + divider
	}
	if header != "" {
		headerHeight = lipgloss.Height(header)
	}

	var body []string
	if p.subtitle != "" {
		body = append(body, textStyle.Render(p.subtitle))
	}
	body = append(body, p.styledLines...)
	diffWidth := max(width-6, 20)
	for _, dl := range p.diffLines {
		switch {
		case dl == "":
			body = append(body, "")
		case strings.HasPrefix(dl, "- "):
			body = append(body, diffOldStyle.Render(diffCell(dl, diffWidth)))
		default:
			body = append(body, diffNewStyle.Render(diffCell(dl, diffWidth)))
		}
	}
	trimTail := func() {
		for len(body) > 1 && strings.TrimSpace(body[len(body)-1]) == "" {
			body = body[:len(body)-1]
		}
	}
	var tail []string
	appendFooter := func(hint string) {
		if p.back != nil && p.kind != popupOAuth {
			hint = strings.NewReplacer("Esc:cancel", "Esc:back", "Esc:close", "Esc:back").Replace(hint)
		}
		trimTail()
		tail = []string{renderFooter(hint)}
	}

	trimTail()
	body = append(body, "")

	listRows := func(visible int) int {
		if t.height <= 0 {
			return visible
		}
		used := lipgloss.Height(lipgloss.NewStyle().Width(width-2).Render(strings.Join(body, "\n"))) + 2 + headerHeight
		if len(p.questions) > 1 {
			used++
		}
		return max(t.height-used, 1)
	}

	switch p.kind {
	case popupConfirm, popupSingleSelect:
		if p.searchable && len(p.options) == 0 {
			body = append(body, hintStyle.Render("  no matching settings"))
		}
		total := len(p.options)
		visible := p.maxVisible
		if visible <= 0 && p.kind == popupSingleSelect {
			visible = cmdSelectorMaxVisible
		}
		visible = listRows(visible)
		start, end := 0, total
		if visible > 0 && total > visible {
			if p.readOnly {
				start = min(p.cursor, total-visible)
				end = start + visible
			} else {
				start, end = windowRange(p.cursor, total, visible)
			}
		}
		maxLine := max(width-10, 32)
		for i := start; i < end; i++ {
			opt := truncate.StringWithTail(p.options[i], uint(maxLine), "...")
			marker := "  "
			var line string
			if !p.readOnly && i == p.cursor {
				marker = systemStyle.Render("⏵ ")
				head, tail := splitOptStyle(opt)
				line = systemStyle.Render(head)
				if tail != "" {
					line += hintStyle.Render(tail)
				}
			} else if !p.readOnly {
				head, tail := splitOptStyle(opt)
				line = whiteStyle.Render(head)
				if tail != "" {
					line += hintStyle.Render(tail)
				}
			} else {
				line = hintStyle.Render(opt)
			}
			if i < len(p.optionTail) && p.optionTail[i] != "" {
				line += " " + p.optionTail[i]
			}
			body = append(body, marker+line)
		}
		action := "confirm"
		if p.enterAction != "" {
			action = p.enterAction
		}
		hint := "Enter:" + action + "  Esc:cancel"
		if p.searchable {
			hint = "Enter:" + action + "  Esc:close"
			if p.input.Value() != "" {
				hint = "Enter:" + action + "  Esc:clear"
			}
		}
		if p.readOnly {
			hint = "Esc:close"
		}
		if p.onDelete != nil {
			hint += "  d:delete"
		}
		if p.onTag != nil {
			hint += "  t:tier tag"
		}
		if p.onMove != nil {
			hint += "  w/s:fallback order"
		}
		if p.link() != "" {
			hint += "  o:" + p.openLabel
		}
		appendFooter(hint)

	case popupMultiSelect:
		if p.searchable && len(p.options) == 0 {
			body = append(body, hintStyle.Render("  no matches"))
		}
		total := len(p.options)
		visible := p.maxVisible
		if visible <= 0 {
			visible = cmdSelectorMaxVisible
		}
		visible = listRows(visible)
		start, end := windowRange(p.cursor, total, visible)
		maxLine := max(width-14, 32)
		for i := start; i < end; i++ {
			opt := truncate.StringWithTail(p.options[i], uint(maxLine), "...")
			cursor := "  "
			head, tail := splitOptStyle(opt)
			var line string
			if i == p.cursor {
				cursor = systemStyle.Render("⏵ ")
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
		hint := "Space:toggle  Enter:confirm  Esc:cancel"
		if p.searchable && p.input.Value() != "" {
			hint = "Space:toggle  Enter:confirm  Esc:clear"
		}
		appendFooter(hint)

	case popupText:
		p.input.SetWidth(max(width-10, 20))
		body = append(body, p.input.View())
		if p.multiline {
			appendFooter("Ctrl+s:confirm  Enter:newline  Esc:cancel")
		} else {
			appendFooter("Enter:confirm  Esc:cancel")
		}

	case popupSecret:
		p.input.SetWidth(max(width-10, 20))
		mask := strings.Repeat("•", len([]rune(p.input.Value())))
		secret := newPopupInput(mask, false)
		cursor := p.input.LineInfo().StartColumn + p.input.LineInfo().CharOffset
		secret.SetCursor(cursor)
		secret.SetWidth(max(width-10, 20))
		body = append(body, secret.View())
		appendFooter("Enter:confirm  Esc:cancel  (input hidden)")

	case popupOAuth:
		if p.oauth != nil {
			if p.oauth.url != "" {
				body = append(body, hintStyle.Render("url:  ")+textStyle.Render(p.oauth.url))
			}
			if p.oauth.userCode != "" {
				body = append(body, hintStyle.Render("code: ")+systemStyle.Render(p.oauth.userCode))
			}
		}
		if p.oauth != nil && p.oauth.mcpServer != "" {
			appendFooter("Enter:re-open browser  p:paste redirect URL  Esc:cancel")
		} else {
			appendFooter("Enter:re-open browser  Esc:cancel")
		}
	}

	if len(p.questions) > 1 {
		footer := hintStyle.Render(fmt.Sprintf("question %d/%d", p.questionIdx+1, len(p.questions)))
		tail = append(tail, footer)
	}

	content := popupStyle.Width(width).Render(strings.Join(body, "\n"))
	if header != "" {
		content = header + "\n" + content
	}
	footer, footerHeight := "", 0
	if len(tail) > 0 {
		footer = divider + "\n" + popupStyle.Width(width).Render(strings.Join(tail, "\n"))
		footerHeight = lipgloss.Height(footer)
	}

	if gap := t.height - lipgloss.Height(content) - footerHeight; t.height > 0 && gap > 0 {
		content += strings.Repeat("\n", gap)
	}
	if footer == "" {
		return content
	}
	return content + "\n" + footer
}

func (t TUI) modelTag() string {
	sid := strings.TrimSpace(t.currentSessionID)
	if sid == "" {
		return ""
	}

	model, reasoning := configbot.GetModel(sid)
	modelPart := hintStyle.Render(model)
	if model != configbot.DefaultModel {
		modelPart = warnStyle.Render(model)
	}

	var reasonPart string
	switch reasoning {
	case "none", "low":
		reasonPart = okayStyle.Render(reasoning)
	case "high", "xhigh", "max":
		reasonPart = errorStyle.Render(reasoning)
	default:
		reasonPart = hintStyle.Render(reasoning)
	}
	return modelPart + hintStyle.Render("/") + reasonPart
}

func renderFooter(hint string) string {
	var items []string
	for item := range strings.SplitSeq(hint, "  ") {
		key, label, ok := strings.Cut(item, ":")
		if !ok {
			items = append(items, hintStyle.Render(item))
			continue
		}
		items = append(items, keyStyle.Render(key)+hintStyle.Render(":"+label))
	}
	return strings.Join(items, hintStyle.Render(" | "))
}

var pendingCache struct {
	sync.Mutex
	sessionID string
	count     int
	at        time.Time
}

func pendingCount(sessionID string) int {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return 0
	}
	pendingCache.Lock()
	defer pendingCache.Unlock()
	if pendingCache.sessionID != sessionID || time.Since(pendingCache.at) > 2*time.Second {
		pendingCache.sessionID = sessionID
		pendingCache.count = len(interactive.ListResumablePending(sessionID))
		pendingCache.at = time.Now()
	}
	return pendingCache.count
}

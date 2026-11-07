package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	goruntime "runtime"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type popupType int

const (
	popupConfirm popupType = iota
	popupText
	popupSecret
	popupSingleSelect
	popupMultiSelect
	popupOAuth
)

type Popup struct {
	pendingId string

	kind        popupType
	title       string
	subtitle    string
	styledLines []string
	diffLines   []string

	options     []string
	optionTail  []string
	values      []string
	cursor      int
	multi       map[int]bool
	onToggle    func(p *Popup, index int)
	maxVisible  int
	readOnly    bool
	enterAction string

	tabs   []string
	tabIdx int
	onTab  func(p *Popup) tea.Cmd

	searchable bool
	allOptions []string
	allValues  []string
	allMulti   map[int]bool
	indexes    []int

	input          textarea.Model
	multiline      bool
	skipWithReason bool
	restricted     []string
	needPassword   bool
	passwordPrompt bool
	explain        string
	commandLine    bool
	status         string
	verifying      bool

	questions   []runtime.Question
	questionIdx int
	answers     []any

	onConfirm func(chosen string) any
	onDelete  func(chosen string) any
	onTag     func(chosen string) any
	onMove    func(chosen, neighbor string) error
	onOpen    func(chosen string) string
	openLabel string
	onEnter   func(p *Popup, chosen string) tea.Cmd
	onCancel  func() any
	back      *Popup

	oauth *oauthState
}

type oauthState struct {
	provider  string
	mcpServer string
	url       string
	userCode  string
	cancel    context.CancelFunc
}

func (t TUI) closePopup() TUI {
	t.popup = nil
	for len(t.popupQueue) > 0 {
		next := t.popupQueue[0]
		t.popupQueue = t.popupQueue[1:]
		if ps := newPopup(next); ps != nil {
			t.popup = ps
			return t
		}
		resolvePending(next.id, runtime.Reply{
			Error: utilsmsg.New(utilsmsg.Invalid, "pending request", "no popup for this request"),
		})
	}
	return t
}

func (t TUI) updatePopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch t.popup.kind {
	case popupConfirm:
		return t.updateConfirmPopup(msg)

	case popupSingleSelect:
		return t.updateSingleSelectPopup(msg)

	case popupMultiSelect:
		return t.updateMultiSelectPopup(msg)

	case popupText, popupSecret:
		return t.updateTextInputPopup(msg)

	case popupOAuth:
		return t.updateOAuthPopup(msg)
	}
	return t, nil
}

func (t TUI) updateOAuthPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	if p.oauth == nil {
		return t, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		if p.oauth.cancel != nil {
			p.oauth.cancel()
		}
	case tea.KeyEnter:
		if p.oauth.url != "" {
			openBrowser(p.oauth.url)
		}
	case tea.KeyRunes:
		if p.oauth.mcpServer != "" && strings.EqualFold(string(msg.Runes), "p") {
			return t.openMcpOAuthPaste(p.oauth)
		}
	}
	return t, nil
}

func openBrowser(link string) {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "linux":
		cmd = linuxOpenCommand(link)
	default:
		return
	}
	if cmd == nil {
		slog.Warn("openOAuthBrowser: no opener available",
			slog.String("url", link))
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("openOAuthBrowser cmd.Start",
			slog.String("url", link),
			slog.String("error", err.Error()))
	}
}

func linuxOpenCommand(link string) *exec.Cmd {
	if bin := utils.WSLChromePath(); bin != "" {
		return exec.Command(bin, link)
	}
	if bin, err := exec.LookPath("wslview"); err == nil {
		return exec.Command(bin, link)
	}
	if bin, err := exec.LookPath("xdg-open"); err == nil {
		return exec.Command(bin, link)
	}
	return nil
}

func (t TUI) updateConfirmPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown, tea.KeyTab:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeyEsc:
		return t, nil

	case tea.KeyEnter:
		chosen := p.options[p.cursor]
		var reply runtime.Reply
		switch {
		case chosen == "Yes":
			if len(p.restricted) > 0 {
				if !p.needPassword {
					resolvePending(p.pendingId, runtime.Reply{Approve: true})
					t = t.closePopup()
					return t, nil
				}
				p.kind = popupSecret
				p.passwordPrompt = true
				p.title = "System password:"
				p.styledLines = nil
				for _, one := range p.restricted {
					p.styledLines = append(p.styledLines, warnStyle.Render("⚠ "+one))
				}
				p.input = newPopupInput("", false)
				return t, nil
			}
			reply = runtime.Reply{Approve: true}
		case strings.HasPrefix(chosen, "Yes  don't ask again"):
			reply = runtime.Reply{Approve: true, Remember: true}
		case strings.HasPrefix(chosen, "Yes  allow this turn"):
			reply = runtime.Reply{Approve: true, AllowTurn: true}
		case chosen == confirmSkip:
			p.kind = popupText
			p.skipWithReason = true
			p.title = "Reason (enter to skip):"
			p.input = newPopupInput("", false)
			return t, nil
		case chosen == confirmAbort:
			reply = runtime.Reply{Approve: false, Error: runtime.ErrUserAborted}
		case chosen == confirmCancel:
			reply = runtime.Reply{Approve: false, Error: runtime.ErrUserCanceled}
		}
		resolvePending(p.pendingId, reply)
		t = t.closePopup()
	}
	return t, nil
}

func (t TUI) updateSingleSelectPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	if p.searchable {
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyEnter:
		case tea.KeyLeft, tea.KeyRight:
			if len(p.tabs) < 2 {
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				p.filter()
				return t, cmd
			}
		case tea.KeyEsc:
			if p.input.Value() != "" {
				p.input.Reset()
				p.filter()
				return t, nil
			}
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			p.filter()
			return t, cmd
		}
	}
	switch msg.Type {
	case tea.KeyUp:
		if p.readOnly {
			p.scroll(-1)
			break
		}
		p.move(-1)

	case tea.KeyDown:
		if p.readOnly {
			p.scroll(1)
			break
		}
		p.move(1)

	case tea.KeyLeft:
		return t, p.switchTab(-1)

	case tea.KeyRight:
		return t, p.switchTab(1)

	case tea.KeyEsc:
		if p.pendingId == "" {
			return t.escapePopup()
		}
		return t, nil
	case tea.KeyRunes:
		if p.pendingId != "" {
			break
		}
		if step, ok := map[string]int{"w": -1, "s": 1}[strings.ToLower(string(msg.Runes))]; ok {
			p.swap(step)
			break
		}
		if strings.EqualFold(string(msg.Runes), "o") {
			if link := p.link(); link != "" {
				openBrowser(link)
			}
			break
		}
		var action func(chosen string) any
		switch strings.ToLower(string(msg.Runes)) {
		case "d":
			action = p.onDelete
		case "t":
			action = p.onTag
		}
		if action == nil {
			break
		}
		chosen := p.options[p.cursor]
		if p.values != nil && p.cursor < len(p.values) {
			chosen = p.values[p.cursor]
		}
		next := action(chosen)
		if next == nil {
			break
		}
		t = t.closePopup()
		return t, popupNext(p, func() any { return next })

	case tea.KeyEnter:
		if len(p.options) == 0 {
			break
		}
		if !p.readOnly && strings.TrimSpace(p.options[p.cursor]) == "" {
			break
		}
		chosen := p.options[p.cursor]
		if p.values != nil && p.cursor < len(p.values) {
			chosen = p.values[p.cursor]
		}
		if p.pendingId == "" && p.onEnter != nil {
			return t, p.onEnter(p, chosen)
		}
		if p.pendingId == "" {
			cb := p.onConfirm
			t = t.closePopup()
			if cb == nil {
				return t, nil
			}
			return t, popupNext(p, func() any { return cb(chosen) })
		}

		resolved, reply := p.advanceOrResolve(chosen)
		if resolved {
			resolvePending(p.pendingId, reply)
			t = t.closePopup()
		}
	}
	return t, nil
}

type popupChild struct {
	parent *Popup
	msg    tea.Msg
}

func popupNext(parent *Popup, cb func() any) tea.Cmd {
	return func() tea.Msg {
		return popupChild{parent: parent, msg: cb()}
	}
}

func (t TUI) runPopupChild(msg popupChild) (tea.Model, tea.Cmd) {
	if msg.msg == nil {
		return t, nil
	}
	t.popupOrigin = msg.parent
	next, cmd := t.update(msg.msg)
	nt, ok := next.(TUI)
	if !ok {
		return next, cmd
	}
	nt.popupOrigin = nil
	if nt.popup == nil || nt.popup == msg.parent || nt.popup.pendingId != "" || nt.popup.back != nil {
		return nt, cmd
	}
	nt.popup.back = msg.parent
	for one := msg.parent; one != nil; one = one.back {
		if one.title == nt.popup.title {
			nt.popup.back = one.back
			break
		}
	}
	return nt, cmd
}

func (t TUI) escapePopup() (tea.Model, tea.Cmd) {
	if back := t.popup.back; back != nil {
		t.popup = back
		return t, nil
	}
	cb := t.popup.onCancel
	t = t.closePopup()
	if cb != nil {
		return t, func() tea.Msg { return cb() }
	}
	return t, nil
}

func (p *Popup) move(step int) {
	n := len(p.options)
	for range n {
		p.cursor = (p.cursor + step + n) % n
		if strings.TrimSpace(p.options[p.cursor]) != "" {
			return
		}
	}
}

func (p *Popup) link() string {
	if p.onOpen == nil || p.cursor >= len(p.values) {
		return ""
	}
	return p.onOpen(p.values[p.cursor])
}

func (p *Popup) swap(step int) {
	next := p.cursor + step
	if p.onMove == nil || next < 0 || next >= len(p.values) || p.cursor >= len(p.values) {
		return
	}
	if err := p.onMove(p.values[p.cursor], p.values[next]); err != nil {
		p.subtitle = err.Error()
		return
	}
	p.options[p.cursor], p.options[next] = p.options[next], p.options[p.cursor]
	p.values[p.cursor], p.values[next] = p.values[next], p.values[p.cursor]
	p.cursor = next
}

func (p *Popup) filter() {
	if p.kind == popupMultiSelect {
		p.syncMulti()
	}
	query := strings.ToLower(strings.TrimSpace(p.input.Value()))
	p.options = p.options[:0:0]
	p.values = p.values[:0:0]
	p.indexes = p.indexes[:0:0]
	for i, one := range p.allOptions {
		if strings.Contains(strings.ToLower(ansi.Strip(one)), query) {
			p.options = append(p.options, one)
			p.values = append(p.values, p.allValues[i])
			p.indexes = append(p.indexes, i)
		}
	}
	if p.kind == popupMultiSelect {
		p.multi = make(map[int]bool, len(p.indexes))
		for i, idx := range p.indexes {
			if p.allMulti[idx] {
				p.multi[i] = true
			}
		}
	}
	p.cursor = 0
}

func (p *Popup) syncMulti() {
	if p.allMulti == nil {
		p.allMulti = make(map[int]bool, len(p.allOptions))
	}
	for i := range p.options {
		idx := i
		if p.indexes != nil {
			idx = p.indexes[i]
		}
		p.allMulti[idx] = p.multi[i]
	}
}

func (p *Popup) scroll(step int) {
	visible := p.maxVisible
	if visible <= 0 {
		visible = cmdSelectorMaxVisible
	}
	p.cursor = min(max(p.cursor+step, 0), max(len(p.options)-visible, 0))
}

func (p *Popup) switchTab(step int) tea.Cmd {
	if len(p.tabs) < 2 || p.onTab == nil {
		return nil
	}
	p.tabIdx = (p.tabIdx + step + len(p.tabs)) % len(p.tabs)
	return p.onTab(p)
}

func (t TUI) updateMultiSelectPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	if p.searchable {
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeySpace, tea.KeyEnter:
		case tea.KeyLeft, tea.KeyRight:
			if len(p.tabs) < 2 {
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				p.filter()
				return t, cmd
			}
		case tea.KeyEsc:
			if p.input.Value() != "" {
				p.input.Reset()
				p.filter()
				return t, nil
			}
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			p.filter()
			return t, cmd
		}
	}
	if len(p.options) == 0 && (msg.Type == tea.KeyUp || msg.Type == tea.KeyDown || msg.Type == tea.KeySpace) {
		return t, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeyLeft:
		return t, p.switchTab(-1)

	case tea.KeyRight:
		return t, p.switchTab(1)

	case tea.KeySpace:
		p.multi[p.cursor] = !p.multi[p.cursor]
		if p.onToggle != nil {
			p.onToggle(p, p.cursor)
		}

	case tea.KeyEsc:
		if p.pendingId == "" {
			return t.escapePopup()
		}
		return t, nil

	case tea.KeyEnter:
		options, values, multi := p.options, p.values, p.multi
		if p.searchable {
			p.syncMulti()
			options, values, multi = p.allOptions, p.allValues, p.allMulti
		}
		selected := make([]string, 0, len(multi))
		for i := range options {
			if multi[i] {
				v := options[i]
				if values != nil && i < len(values) {
					v = values[i]
				}
				selected = append(selected, v)
			}
		}

		if p.pendingId == "" {
			cb := p.onConfirm
			t = t.closePopup()
			if cb == nil {
				return t, nil
			}
			return t, popupNext(p, func() any { return cb(strings.Join(selected, "\x1F")) })
		}

		resolved, reply := p.advanceOrResolve(selected)
		if resolved {
			resolvePending(p.pendingId, reply)
			t = t.closePopup()
		}
	}
	return t, nil
}

func newPopupInput(value string, multiline bool) textarea.Model {
	input := textarea.New()
	input.CharLimit = 8192
	input.ShowLineNumbers = false
	input.SetHeight(1)
	input.SetValue(value)
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.Focus()
	input.Cursor.Style = whiteStyle
	input.Cursor.SetMode(cursor.CursorStatic)
	input.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return systemStyle.Render("> ")
		}
		return "  "
	})
	if multiline {
		input.SetHeight(6)
	}
	return input
}

func (t TUI) updateTextInputPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	submit := func() (tea.Model, tea.Cmd) {
		value := p.input.Value()
		if p.pendingId == "" {
			cb := p.onConfirm
			t = t.closePopup()
			if cb == nil {
				return t, nil
			}
			return t, popupNext(p, func() any { return cb(value) })
		}
		if p.passwordPrompt {
			if p.verifying {
				return t, nil
			}
			p.verifying = true
			p.status = hintStyle.Render("verifying...")
			resolvePendingWithPassword(p.pendingId, runtime.Reply{Approve: true}, value)
			return t, nil
		}
		if p.skipWithReason {
			resolvePending(p.pendingId, runtime.Reply{
				Approve: false,
				Skip:    true,
				Reason:  strings.TrimSpace(value),
			})
			t = t.closePopup()
			return t, nil
		}
		resolved, reply := p.advanceOrResolve(value)
		if resolved {
			resolvePending(p.pendingId, reply)
			t = t.closePopup()
		}
		return t, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		if p.pendingId == "" {
			return t.escapePopup()
		}
		return t, nil

	case tea.KeyCtrlS:
		if p.multiline {
			return submit()
		}

	case tea.KeyEnter:
		if !p.multiline {
			return submit()
		}

	case tea.KeyLeft:
		if len(p.tabs) > 1 {
			return t, p.switchTab(-1)
		}

	case tea.KeyRight:
		if len(p.tabs) > 1 {
			return t, p.switchTab(1)
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return t, cmd
}

const (
	confirmSkip   = "No  skip this"
	confirmAbort  = "Abort  keep for resume"
	confirmCancel = "Cancel  delete task"
)

func newPopup(pending Pending) *Popup {
	id, req := pending.id, pending.request
	switch req.Kind {
	case runtime.KindToolConfirm:
		display := utils.FormatToolEvent(req.ToolName, req.ToolArgs)
		if display == "" {
			display = req.ToolArgs
		}
		options := []string{
			"Yes",
			"Yes  don't ask again",
			"Yes  allow this turn",
			confirmSkip,
			confirmAbort,
			confirmCancel,
		}
		if len(req.Restricted) > 0 {
			options = []string{
				"Yes",
				confirmSkip,
				confirmAbort,
				confirmCancel,
			}
		}
		p := &Popup{
			pendingId:    id,
			kind:         popupConfirm,
			title:        fmt.Sprintf("Run %s?", utils.ToolName(req.ToolName)),
			subtitle:     display,
			options:      options,
			restricted:   req.Restricted,
			needPassword: pending.needPassword,
		}
		p.explain = req.Explain
		p.commandLine = true
		if len(req.Restricted) > 0 {
			p.title = fmt.Sprintf("Run %s outside the allow list?", utils.ToolName(req.ToolName))
			for _, one := range req.Restricted {
				p.styledLines = append(p.styledLines, warnStyle.Render("⚠ "+one))
			}
			p.styledLines = append(p.styledLines, "")
			if !pending.needPassword {
				p.styledLines = append(p.styledLines, okayStyle.Render("sudo credentials still valid — no password needed"))
			} else {
				p.styledLines = append(p.styledLines, userStyle.Render("system password required — you will be prompted after Yes"))
			}
		}
		switch req.ToolName {
		case "edit_file", "edit_tool", "edit_skill":
			hunks := utils.FormatPatchDiff(req.ToolArgs)
			if len(hunks) == 0 {
				for _, l := range utils.FormatWriteDiff(req.ToolArgs) {
					p.diffLines = append(p.diffLines, "+ "+l)
					if len(p.diffLines) >= 16 {
						break
					}
				}
			}
			remaining := 32
			for i, h := range hunks {
				if remaining <= 0 {
					break
				}
				if i > 0 {
					p.diffLines = append(p.diffLines, "")
				}
				for j, l := range h.OldLines[:min(len(h.OldLines), 16, remaining)] {
					p.diffLines = append(p.diffLines, "- "+rowLabel(h.Row, j)+l)
					remaining--
				}
				for j, l := range h.NewLines[:min(len(h.NewLines), remaining)] {
					p.diffLines = append(p.diffLines, "+ "+rowLabel(h.Row, j)+l)
					remaining--
				}
			}
		}
		return p
	case runtime.KindAskUser:
		if req.AskUser == nil || len(req.AskUser.Questions) == 0 {
			return nil
		}
		ps := &Popup{
			pendingId:   id,
			questions:   req.AskUser.Questions,
			questionIdx: 0,
			answers:     make([]any, 0, len(req.AskUser.Questions)),
		}
		ps.loadCurrentQuestion()
		return ps
	}
	return nil
}

func (p *Popup) loadCurrentQuestion() {
	q := p.questions[p.questionIdx]
	p.title = q.Question
	p.subtitle = q.Detail
	p.input = newPopupInput("", false)
	p.multi = nil

	switch {
	case len(q.Options) == 0 && q.Secret:
		p.kind = popupSecret
		p.maxVisible = 0
	case len(q.Options) == 0:
		p.kind = popupText
		p.maxVisible = 0
	case q.MultiSelect:
		p.kind = popupMultiSelect
		p.options = q.Options
		p.multi = make(map[int]bool, len(q.Options))
		p.maxVisible = cmdSelectorMaxVisible
	default:
		p.kind = popupSingleSelect
		p.options = q.Options
		p.maxVisible = cmdSelectorMaxVisible
	}
}

func (p *Popup) advanceOrResolve(answer any) (resolved bool, reply runtime.Reply) {
	p.answers = append(p.answers, answer)
	p.questionIdx++
	if p.questionIdx >= len(p.questions) {
		return true, runtime.Reply{Answers: p.answers}
	}
	p.loadCurrentQuestion()
	return false, runtime.Reply{}
}

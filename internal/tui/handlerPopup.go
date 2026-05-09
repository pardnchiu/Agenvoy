package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/pending"
)

type popupType int

const (
	popupConfirm popupType = iota
	popupText
	popupSecret
	popupSingleSelect
	popupMultiSelect
)

type Popup struct {
	pendingId string

	kind     popupType
	title    string
	subtitle string

	options []string
	values  []string
	cursor  int
	multi   map[int]bool

	input string

	questions   []pending.Question
	questionIdx int
	answers     []any

	onConfirm func(chosen string) any
}

func (t TUI) closePopup() TUI {
	t.popup = nil
	for len(t.popupQueue) > 0 {
		next := t.popupQueue[0]
		t.popupQueue = t.popupQueue[1:]
		if ps := newPopup(next.id, next.request); ps != nil {
			t.popup = ps
			return t
		}
		pending.Resolve(next.id, pending.Reply{
			Error: fmt.Errorf("invalid pending request"),
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
	}
	return t, nil
}

func (t TUI) updateConfirmPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown, tea.KeyTab:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeyEsc:
		pending.Resolve(p.pendingId, pending.Reply{
			Approve: false,
			Error:   fmt.Errorf("user stopped"),
		})
		t = t.closePopup()

	case tea.KeyEnter:
		var reply pending.Reply
		switch p.cursor {
		case 0:
			reply = pending.Reply{
				Approve: true,
			}

		case 1:
			reply = pending.Reply{
				Approve: false,
				Skip:    true,
			}

		case 2:
			reply = pending.Reply{
				Approve: false,
				Error:   fmt.Errorf("user stopped"),
			}
		}
		pending.Resolve(p.pendingId, reply)
		t = t.closePopup()

	default:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				pending.Resolve(p.pendingId, pending.Reply{
					Approve: true,
				})
				t = t.closePopup()
			case 's', 'S':
				pending.Resolve(p.pendingId, pending.Reply{
					Approve: false,
					Skip:    true,
				})
				t = t.closePopup()
			case 'n', 'N':
				pending.Resolve(p.pendingId, pending.Reply{
					Approve: false,
					Error:   fmt.Errorf("user stopped"),
				})
				t = t.closePopup()
			}
		}
	}
	return t, nil
}

func (t TUI) updateSingleSelectPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyUp:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeyEsc:
		if p.pendingId == "" {
			t = t.closePopup()
		} else {
			pending.Resolve(p.pendingId, pending.Reply{
				Error: fmt.Errorf("user cancelled"),
			})
			t = t.closePopup()
		}

	case tea.KeyEnter:
		chosen := p.options[p.cursor]
		if p.values != nil && p.cursor < len(p.values) {
			chosen = p.values[p.cursor]
		}
		if p.pendingId == "" {
			cb := p.onConfirm
			t = t.closePopup()
			if cb == nil {
				return t, nil
			}
			return t, func() tea.Msg { return cb(chosen) }
		}

		resolved, reply := p.advanceOrResolve(chosen)
		if resolved {
			pending.Resolve(p.pendingId, reply)
			t = t.closePopup()
		}
	}
	return t, nil
}

func (t TUI) updateMultiSelectPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyUp:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeySpace:
		p.multi[p.cursor] = !p.multi[p.cursor]

	case tea.KeyEsc:
		pending.Resolve(p.pendingId, pending.Reply{
			Error: fmt.Errorf("user cancelled"),
		})
		t = t.closePopup()

	case tea.KeyEnter:
		selected := make([]string, 0, len(p.multi))
		for i, opt := range p.options {
			if p.multi[i] {
				selected = append(selected, opt)
			}
		}

		resolved, reply := p.advanceOrResolve(selected)
		if resolved {
			pending.Resolve(p.pendingId, reply)
			t = t.closePopup()
		}
	}
	return t, nil
}

func (t TUI) updateTextInputPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyEsc:
		pending.Resolve(p.pendingId, pending.Reply{
			Error: fmt.Errorf("user cancelled"),
		})
		t = t.closePopup()

	case tea.KeyEnter:
		resolved, reply := p.advanceOrResolve(p.input)
		if resolved {
			pending.Resolve(p.pendingId, reply)
			t = t.closePopup()
		}

	case tea.KeyBackspace:
		if r := []rune(p.input); len(r) > 0 {
			p.input = string(r[:len(r)-1])
		}

	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			p.input += string(msg.Runes)
		}
	}
	return t, nil
}

func newPopup(id string, req pending.Request) *Popup {
	switch req.Kind {
	case pending.KindToolConfirm:
		return &Popup{
			pendingId: id,
			kind:      popupConfirm,
			title:     fmt.Sprintf("Run %s?", req.ToolName),
			subtitle:  truncate(req.ToolArgs, 200),
			options: []string{
				"Yes",
				"Skip",
				"Stop",
			},
		}
	case pending.KindAskUser:
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
	p.subtitle = ""
	p.input = ""
	p.cursor = 0
	p.multi = nil

	switch {
	case len(q.Options) == 0 && q.Secret:
		p.kind = popupSecret
	case len(q.Options) == 0:
		p.kind = popupText
	case q.MultiSelect:
		p.kind = popupMultiSelect
		p.options = q.Options
		p.multi = make(map[int]bool, len(q.Options))
	default:
		p.kind = popupSingleSelect
		p.options = q.Options
	}
}

func (p *Popup) advanceOrResolve(answer any) (resolved bool, reply pending.Reply) {
	p.answers = append(p.answers, answer)
	p.questionIdx++
	if p.questionIdx >= len(p.questions) {
		return true, pending.Reply{Answers: p.answers}
	}
	p.loadCurrentQuestion()
	return false, pending.Reply{}
}

func truncate(s string, max int) string {
	out := []rune(s)
	for i, r := range out {
		if r == '\n' || r == '\r' {
			out[i] = ' '
		}
	}
	if len(out) > max {
		return string(out[:max]) + "…"
	}
	return string(out)
}

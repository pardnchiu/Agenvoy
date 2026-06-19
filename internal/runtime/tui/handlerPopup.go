package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	goruntime "runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/utils"
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

	kind     popupType
	title    string
	subtitle string

	options    []string
	values     []string
	cursor     int
	multi      map[int]bool
	maxVisible int

	input          string
	multiline      bool
	skipWithReason bool

	questions   []runtime.Question
	questionIdx int
	answers     []any

	onConfirm func(chosen string) any

	oauth *oauthState
}

type oauthState struct {
	provider string
	url      string
	userCode string
	cancel   context.CancelFunc
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
		runtime.Resolve(next.id, runtime.Reply{
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
	}
	return t, nil
}

func openBrowser(link string) {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	default:
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("openOAuthBrowser cmd.Start",
			slog.String("url", link),
			slog.String("error", err.Error()))
	}
}

func (t TUI) updateConfirmPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		p.cursor = (p.cursor - 1 + len(p.options)) % len(p.options)

	case tea.KeyDown, tea.KeyTab:
		p.cursor = (p.cursor + 1) % len(p.options)

	case tea.KeyEsc:
		runtime.Resolve(p.pendingId, runtime.Reply{
			Approve: false,
			Error:   fmt.Errorf("user stopped"),
		})
		t = t.closePopup()

	case tea.KeyEnter:
		if p.cursor == 4 {
			p.kind = popupText
			p.skipWithReason = true
			p.title = "Reason (Enter to skip without reason):"
			p.input = ""
			return t, nil
		}
		var reply runtime.Reply
		switch p.cursor {
		case 0:
			reply = runtime.Reply{
				Approve: true,
			}

		case 1:
			reply = runtime.Reply{
				Approve:  true,
				Remember: true,
			}

		case 2:
			reply = runtime.Reply{
				Approve:   true,
				AllowTurn: true,
			}

		case 3:
			reply = runtime.Reply{
				Approve: false,
				Skip:    true,
			}

		case 5:
			reply = runtime.Reply{
				Approve: false,
				Error:   fmt.Errorf("user stopped"),
			}
		}
		runtime.Resolve(p.pendingId, reply)
		t = t.closePopup()
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
			runtime.Resolve(p.pendingId, runtime.Reply{
				Error: fmt.Errorf("user cancelled"),
			})
			t = t.closePopup()
		}
		if t.onceCall && t.currentSessionID == "" {
			t.quitting = true
			return t, tea.Quit
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
			runtime.Resolve(p.pendingId, reply)
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
		if p.pendingId == "" {
			t = t.closePopup()
		} else {
			runtime.Resolve(p.pendingId, runtime.Reply{
				Error: fmt.Errorf("user cancelled"),
			})
			t = t.closePopup()
		}

	case tea.KeyEnter:
		selected := make([]string, 0, len(p.multi))
		for i := range p.options {
			if p.multi[i] {
				v := p.options[i]
				if p.values != nil && i < len(p.values) {
					v = p.values[i]
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
			return t, func() tea.Msg { return cb(strings.Join(selected, "\x1F")) }
		}

		resolved, reply := p.advanceOrResolve(selected)
		if resolved {
			runtime.Resolve(p.pendingId, reply)
			t = t.closePopup()
		}
	}
	return t, nil
}

func (t TUI) updateTextInputPopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := t.popup
	submit := func() (tea.Model, tea.Cmd) {
		if p.pendingId == "" {
			cb := p.onConfirm
			value := p.input
			t = t.closePopup()
			if cb == nil {
				return t, nil
			}
			return t, func() tea.Msg { return cb(value) }
		}
		if p.skipWithReason {
			runtime.Resolve(p.pendingId, runtime.Reply{
				Approve: false,
				Skip:    true,
				Reason:  strings.TrimSpace(p.input),
			})
			t = t.closePopup()
			return t, nil
		}
		resolved, reply := p.advanceOrResolve(p.input)
		if resolved {
			runtime.Resolve(p.pendingId, reply)
			t = t.closePopup()
		}
		return t, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		if p.pendingId == "" {
			t = t.closePopup()
			if t.onceCall && t.currentSessionID == "" {
				t.quitting = true
				return t, tea.Quit
			}
			return t, nil
		}
		runtime.Resolve(p.pendingId, runtime.Reply{
			Error: fmt.Errorf("user cancelled"),
		})
		t = t.closePopup()

	case tea.KeyCtrlS:
		if p.multiline {
			return submit()
		}

	case tea.KeyEnter:
		if p.multiline {
			p.input += "\n"
			return t, nil
		}
		return submit()

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

func newPopup(id string, req runtime.Request) *Popup {
	switch req.Kind {
	case runtime.KindToolConfirm:
		display := utils.FormatToolEvent(req.ToolName, req.ToolArgs)
		if display == "" {
			display = req.ToolArgs
		}
		return &Popup{
			pendingId: id,
			kind:      popupConfirm,
			title:     fmt.Sprintf("Run %s?", req.ToolName),
			subtitle:  go_pkg_utils.TruncateString(display, 256),
			options: []string{
				"Yes",
				"Yes  don't ask again",
				"Yes  allow this turn",
				"No",
				"No   with reason",
				"Stop",
			},
		}
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
	p.input = ""
	p.cursor = 0
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

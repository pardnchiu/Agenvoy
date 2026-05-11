package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/pending"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if t.popup != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			return t.updatePopup(msg)
		case spinner.TickMsg:
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
			return t, tea.Batch(cmds...)
		case Pending:
			t.popupQueue = append(t.popupQueue, msg)
			return t, nil
		}
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.textarea.SetWidth(msg.Width - 4)
		return t, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ exit")+"\n"),
				tea.Quit,
			)

		case tea.KeyEsc:
			if t.selector != nil {
				t.selector = nil
				return t, nil
			}
			if t.running && t.cancelExec != nil {
				t.cancelExec()
				return t, tea.Println(hintStyle.Render("⎯ cancelling…"))
			}

		case tea.KeyUp:
			if t.selector != nil {
				n := len(t.selector.items)
				t.selector.cursor = (t.selector.cursor - 1 + n) % n
				return t, nil
			}

			if !t.running && (t.inputHistoryIdx >= 0 || t.textarea.Line() == 0) {
				if next, handled := t.clickUp(); handled {
					return next, nil
				}
			}

		case tea.KeyDown:
			if t.selector != nil {
				n := len(t.selector.items)
				t.selector.cursor = (t.selector.cursor + 1) % n
				return t, nil
			}

			if !t.running && (t.inputHistoryIdx >= 0 || t.textarea.Line() == t.textarea.LineCount()-1) {
				if next, handled := t.clickDown(); handled {
					return next, nil
				}
			}

		case tea.KeyTab:
			if t.selector != nil {
				t = t.selectCommand()
				return t, nil
			}

		case tea.KeyEnter:
			if t.selector != nil {
				t = t.selectCommand()
				return t, nil
			}

			if msg.Alt {
				t.textarea.InsertRune('\n')
				t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
				return t, nil
			}

			if t.running {
				if strings.TrimSpace(t.textarea.Value()) == "" {
					return t, nil
				}
				return t, tea.Println(hintStyle.Render("⎯ busy · esc to cancel · queue comming soon"))
			}

			content := strings.TrimSpace(t.textarea.Value())
			if content == "" {
				return t, nil
			}
			t = t.recordInputHistory(content)
			t.textarea.Reset()
			t.textarea.SetHeight(1)

			if strings.HasPrefix(content, "/") {
				if next, cmd, handled := t.handleCommand(content); handled {
					return next, cmd
				}
			}

			t.running = true
			t.runStartedAt = time.Now()
			t.runTarget = targetSession(content, t.currentSessionID)

			go runExec(t.ctx, content, false, t.cwd, t.currentSessionID, t.mode == webMode)

			cmds = append(cmds,
				tea.Println(messageBlock(content)),
				t.spinner.Tick,
			)
			return t, tea.Batch(cmds...)
		}

	case agentExec:
		t.cancelExec = msg.cancel
		return t, nil

	case agentExecDone:
		t.running = false
		t.cancelExec = nil
		t.activity = ""
		t.runTarget = ""
		t.streaming = false
		if t.currentSessionID != "" {
			t.currentSessionName, _ = session.GetBot(t.currentSessionID)
		}
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] exec error: %v", msg.err)) + "\n")
		}
		return t, nil

	case WorkDir:
		t.cwd = msg.dir
		return t, nil

	case agentEvent:
		return t.handleAgentEvent(msg.event)

	case Pending:
		popup := newPopup(msg.id, msg.request)
		if popup == nil {
			pending.Resolve(msg.id, pending.Reply{Error: fmt.Errorf("invalid pending request")})
			return t, nil
		}

		t.popup = popup
		return t, nil

	case ModeSelect:
		return t.runModeSelect(msg.mode)

	case SessionSelect:
		next, cmd := t.runCommandSwitch(msg.id)
		return next, cmd

	case SessionNew:
		next, cmd, _ := t.commandNew(nil)
		return next, cmd

	case ModelScopeSelect:
		switch msg.scope {
		case "global":
			next, cmd := t.openModelGlobalPopup()
			return next, cmd
		case "session":
			next, cmd, _ := t.commandSessionModel()
			return next, cmd
		}
		return t, nil

	case ModelAction:
		switch msg.action {
		case "add":
			next, cmd, _ := t.commandModelAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandModelRemove()
			return next, cmd
		}
		return t, nil

	case ReasoningScopeSelect:
		switch msg.scope {
		case "global":
			next, cmd := t.openReasoningGlobalPopup()
			return next, cmd
		case "session":
			next, cmd := t.openReasoningSessionPopup()
			return next, cmd
		}
		return t, nil

	case ModelRemove:
		next, cmd := t.runModelRemove(msg.name)
		host.Reload()
		return next, cmd

	case BotEditDone:
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.discordStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] bot edit: %v", msg.err))+"\n"))
		}
		return t, tea.Sequence(seq...)

	case ModelAddDone:
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.discordStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] add-model: %v", msg.err))+"\n"))
		} else {
			host.Reload()
			seq = append(seq, tea.Println(hintStyle.Render("⎯ model added · registry reloaded")+"\n"))
		}
		return t, tea.Sequence(seq...)

	case DiscordAction:
		return t, runDiscordAction(msg.action)

	case DiscordDone:
		t.discordStatus = getDiscordStatus()
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.discordStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] discord %s: %v", msg.action, msg.err))+"\n"))
		} else {
			seq = append(seq, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ discord %sd · daemon reloading", msg.action))+"\n"))
		}
		return t, tea.Sequence(seq...)

	case PlannerSelect:
		next, cmd := t.runPlannerSelect(msg.name)
		host.Reload()
		return next, cmd

	case ReasoningSelect:
		next, cmd := t.runReasoningSelect(msg.level)
		return next, cmd

	case SessionModelSelect:
		next, cmd := t.runSessionModelSelect(msg.model)
		return next, cmd

	case SessionReasoningSelect:
		next, cmd := t.runSessionReasoningSelect(msg.reasoning)
		return next, cmd

	case UpdateConfirm:
		if !msg.ok {
			return t, tea.Println(hintStyle.Render("⎯ update cancelled") + "\n")
		}
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ stopping daemon · downloading latest · expect sudo prompt")+"\n"),
			runUpdateExec(),
		)

	case UpdateDone:
		t.quitting = true
		if msg.err != nil {
			return t, tea.Sequence(
				tea.Println(errorStyle.Render(fmt.Sprintf("[!] update: %v", msg.err))+"\n"),
				tea.Quit,
			)
		}
		return t, tea.Quit

	case LoadHistoryCheck:
		sid := msg.id
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   "Load previous session history?",
			options: []string{"Yes", "No"},
			values:  []string{"yes", "no"},
			cursor:  1,
			onConfirm: func(chosen string) any {
				return LoadHistorySelect{id: sid, load: chosen == "yes"}
			},
		}
		return t, nil

	case LoadHistorySelect:
		if !msg.load {
			return t, nil
		}
		return t, tea.Sequence(loadSessionTail(msg.id)...)

	case tailLine:
		if t.mode != cliMode {
			return t, nil
		}
		return t, tea.Println(msg.line)

	case initTailer:
		return t.restartTailer(), nil

	case released:
		if msg.tag == "" || msg.tag == projectVersion || projectVersion == "dev" {
			return t, nil
		}

		hint := okayStyle.Render("⏺ latest: "+msg.tag) + hintStyle.Render("  (now is ") + textStyle.Render(projectVersion) + hintStyle.Render(")")
		return t, tea.Println(hint + "\n")

	case spinner.TickMsg:
		if t.running {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	prev := t.textarea.Value()

	var cmd tea.Cmd
	t.textarea, cmd = t.textarea.Update(msg)
	cmds = append(cmds, cmd)
	t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
	if t.textarea.Value() != prev {
		t.inputHistoryIdx = -1
		t = t.refreshCmdSelector()
	}

	return t, tea.Batch(cmds...)
}

package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type agentEvent struct {
	event agentTypes.Event
}

type agentExec struct {
	cancel context.CancelFunc
}

type agentExecDone struct {
	err error
}

func truncatePushPrefix(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return string(r)
}

func runExec(parentCtx context.Context, input string, allowAll bool, workDir, sessionID string, webMode bool) {
	ctx, cancel := context.WithCancel(exec.WithDcPushPrefix(parentCtx, truncatePushPrefix(input, 32)))
	send(agentExec{cancel: cancel})

	ch := make(chan agentTypes.Event, 16)
	done := make(chan error, 1)

	scanner := host.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	go func() {
		err := exec.Run(
			ctx,
			host.Planner(),
			host.Registry(),
			scanner,
			input,
			nil,
			nil,
			ch,
			allowAll,
			workDir,
			sessionID,
			webMode,
		)
		close(ch)
		done <- err
	}()

	for ev := range ch {
		send(agentEvent{event: ev})
	}

	err := <-done
	send(agentExecDone{err: err})
}

func (t TUI) handleAgentEvent(ev agentTypes.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case agentTypes.EventAgentSelect:
		if ev.Source == "" {
			t.activity = "selecting agent…"
		}

	case agentTypes.EventAgentResult:
		if ev.Source == "" {
			text := strings.TrimSpace(ev.Text)
			t.currentModel = text
			t.activity = text
		}

	case agentTypes.EventToolCall:
		if ev.ToolName != "" && ev.ToolName != "ask_user" && ev.ToolName != "store_secret" {
			t.activity = "tool: " + ev.ToolName
		}

	case agentTypes.EventSummaryGenerate:
		t.activity = "summarizing…"

	case agentTypes.EventText:
		if ev.Source == "" {
			line := ev.Text
			var rendered string
			if !t.streaming {
				t.streaming = true
				t.activity = "responding"
				prefix := systemStyle.Render("⏺ ")
				if strings.TrimSpace(t.runTarget) != "" {
					prefix = warnStyle.Render("⏺ [" + t.runTarget + "] ")
				}
				rendered = prefix + line
			} else {
				rendered = "  " + line
			}
			return t, tea.Println(rendered)
		}

	case agentTypes.EventTextDone:
		if ev.Source == "" {
			t.streaming = false
		}
		return t, nil

	case agentTypes.EventDone:
		if ev.Usage != nil {
			t.tokens = ev.Usage.Input + ev.Usage.Output
			t.lastIn = ev.Usage.Input
			t.lastOut = ev.Usage.Output
		}
	}

	line, ok := renderAgentEvent(ev, t.runTarget, t.cwd)
	if !ok {
		return t, nil
	}
	return t, tea.Println(line)
}

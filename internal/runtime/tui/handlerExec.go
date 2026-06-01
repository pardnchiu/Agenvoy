package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
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
	wrapped := wrapEventsPublish(ctx, sessionID, ch)
	done := make(chan error, 1)

	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				close(wrapped)
				done <- fmt.Errorf("exec.Run panic: %v", r)
			}
		}()
		err := exec.Run(
			ctx,
			agents.DispatcherBot(),
			agents.Registry(),
			scanner,
			input,
			nil,
			nil,
			wrapped,
			allowAll,
			workDir,
			sessionID,
			webMode,
		)
		close(wrapped)
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
			str := strings.TrimSpace(ev.Text)
			t.currentModel = str
			t.activity = str
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

package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

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

func runExec(parentCtx context.Context, input string, allowAll bool, workDir, sessionID, pendingTask string) {
	ctx, cancel := context.WithCancel(exec.WithDcPushPrefix(parentCtx, go_pkg_utils.TruncateString(input, 32)))
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
			pendingTask,
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
		if ev.ToolName != "" && ev.ToolName != "ask_user" && ev.ToolName != "store_secret" &&
			ev.ToolName != "list_recent_tool_call" && ev.ToolName != "read_tool_call" {
			t.activity = "tool: " + ev.ToolName
		}

	case agentTypes.EventSummaryGenerate:
		t.activity = "summarizing…"

	case agentTypes.EventText:
		if ev.Source == "" {
			raw := ev.Text

			if len(t.tableBuf) > 0 {
				if strings.Contains(raw, "|") {
					t.tableBuf = append(t.tableBuf, raw)
					return t, nil
				}
				cmds := t.flushTableBuf()
				cmds = append(cmds, t.printStreamLine(renderMarkdown(raw)))
				return t, tea.Batch(cmds...)
			}

			if strings.Contains(raw, "|") {
				t.tableBuf = append(t.tableBuf, raw)
				return t, nil
			}

			return t, t.printStreamLine(renderMarkdown(raw))
		}

	case agentTypes.EventTextDone:
		if ev.Source == "" {
			var cmd tea.Cmd
			if len(t.tableBuf) > 0 {
				cmd = tea.Batch(t.flushTableBuf()...)
			}
			t.streaming = false
			return t, cmd
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

func (t *TUI) printStreamLine(line string) tea.Cmd {
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
	return tea.Println(rendered)
}

func (t *TUI) flushTableBuf() []tea.Cmd {
	block := strings.Join(t.tableBuf, "\n")
	t.tableBuf = nil

	rendered := renderTables(block)
	rendered = renderMarkdown(rendered)

	var sb strings.Builder
	for i, line := range strings.Split(rendered, "\n") {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i == 0 && !t.streaming {
			t.streaming = true
			t.activity = "responding"
			if strings.TrimSpace(t.runTarget) != "" {
				sb.WriteString(warnStyle.Render("⏺ [" + t.runTarget + "] "))
			} else {
				sb.WriteString(systemStyle.Render("⏺ "))
			}
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(line)
	}
	return []tea.Cmd{tea.Println(sb.String() + "\n")}
}

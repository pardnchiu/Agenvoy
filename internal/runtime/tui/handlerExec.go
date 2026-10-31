package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/guide"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/ipc"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type agentEvent struct {
	event agenttypes.Event
}

func (t *TUI) collapseToolBuf() tea.Cmd {
	count, subs := t.toolCount, t.subCount
	t.toolBuf = nil
	t.toolCount = 0
	t.subCount = 0
	if count == 0 && subs == 0 {
		return nil
	}
	var parts []string
	if count > 0 {
		label := "tool call"
		if count != 1 {
			label += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", count, label))
	}
	if subs > 0 {
		label := "subagent call"
		if subs != 1 {
			label += "s"
		}
		parts = append(parts, fmt.Sprintf("%d %s", subs, label))
	}
	return tea.Println(hintStyle.Render("  Ran "+strings.Join(parts, "  ")) + "\n")
}

const subagentLogLines = 3

const commandLogLines = 128

type subagentLive struct {
	tools int
	lines []string
}

func (b *subagentLive) render(name string, width int) string {
	header := "  ⎿ [" + name + "]"
	if b.tools > 0 {
		label := "tool"
		if b.tools != 1 {
			label = "tools"
		}
		header += fmt.Sprintf(" %d %s", b.tools, label)
	}
	var sb strings.Builder
	sb.WriteString(hintStyle.Render(header))
	for _, line := range b.lines {
		sb.WriteByte('\n')
		sb.WriteString(hintStyle.Render(go_pkg_utils.TruncateString("    ⎿  "+line, max(width-4, 32))))
	}
	return sb.String()
}

func (t *TUI) trackSubagent(name, activity string) *subagentLive {
	if t.subBuf == nil {
		t.subBuf = map[string]*subagentLive{}
	}
	block, ok := t.subBuf[name]
	if !ok {
		block = &subagentLive{}
		t.subBuf[name] = block
		t.subOrder = append(t.subOrder, name)
	}
	if activity != "" {
		block.lines = append(block.lines, activity)
		if len(block.lines) > subagentLogLines {
			block.lines = block.lines[len(block.lines)-subagentLogLines:]
		}
	}
	return block
}

func toolActivity(ev agenttypes.Event, cwd string) string {
	label := utils.ToolName(ev.ToolName)
	if arg := utils.FormatToolArgs(ev.ToolName, ev.ToolArgs, cwd); arg != "" {
		label += "(" + arg + ")"
	}
	return oneLine(label)
}

func (t *TUI) dropSubagent(name string) {
	if t.subBuf == nil {
		return
	}
	delete(t.subBuf, name)
	if i := slices.Index(t.subOrder, name); i >= 0 {
		t.subOrder = slices.Delete(t.subOrder, i, i+1)
	}
}

type agentExec struct {
	cancel context.CancelCauseFunc
}

type agentExecDone struct {
	err error
}

const interruptWindow = 3 * time.Second

func runExec(f ipc.Frame) {
	client := ipcClient.Load()
	if client == nil {
		send(agentExecDone{err: ipc.ErrOffline})
		return
	}

	var mu sync.Mutex
	taskHash := ""
	var pendingCancel *bool
	send(agentExec{cancel: func(cause error) {
		mu.Lock()
		defer mu.Unlock()
		pause := cause == nil
		if taskHash == "" {
			pendingCancel = &pause
			return
		}
		client.Cancel(taskHash, pause)
	}})

	f.UUID = go_pkg_utils.UUID()
	f.Rayload.WindowHash = sessiontui.Get()
	f.Rayload.Fast = fast.IsEnabled()
	f.Rayload.Guide = guide.IsEnabled()
	frames, err := client.Run(f)
	if err != nil {
		send(agentExecDone{err: err})
		return
	}

	var result error
	for f := range frames {
		if f.Type == ipc.FrameDone {
			switch {
			case f.Canceled:
				result = context.Canceled
			case f.Error != "":
				result = errors.New(f.Error)
			}
			continue
		}
		ev := *f.Event
		if f.Error != "" {
			ev.Err = errors.New(f.Error)
		}
		if ev.TaskHash != "" {
			mu.Lock()
			if taskHash == "" && pendingCancel != nil {
				client.Cancel(ev.TaskHash, *pendingCancel)
			}
			taskHash = ev.TaskHash
			mu.Unlock()
		}
		send(agentEvent{event: ev})
		switch ev.Type {
		case agenttypes.EventDone, agenttypes.EventReasoning, agenttypes.EventText, agenttypes.EventToolCall, agenttypes.EventCompact:
			time.Sleep(10 * time.Millisecond)
		}
	}
	send(agentExecDone{err: result})
}

func (t TUI) handleAgentEvent(ev agenttypes.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case agenttypes.EventAgentSelect:
		if ev.Source != "" {
			t.trackSubagent(ev.Source, "selecting agent...")
			return t, nil
		}
		t.activity = "selecting agent..."

	case agenttypes.EventAgentResult:
		if ev.Source == "" {
			str := strings.TrimSpace(ev.Text)
			prev := t.currentModel
			t.currentModel = str
			t.activity = str
			if str == "" || str == prev {
				return t, nil
			}
			action := "Model"
			if prev != "" {
				action = "Switch"
			}
			if reasoning := strings.TrimSpace(ev.Reasoning); reasoning != "" {
				str += " " + reasoning
			}
			t.toolBuf = append(t.toolBuf, hintStyle.Render("⏵ "+action+"("+str+")"))
			return t, nil
		}

	case agenttypes.EventTodoUpdate:
		if ev.Source == "" {
			t.todos = ev.Todos
		}
		return t, nil

	case agenttypes.EventUserInjected:
		if ev.Source == "" {
			t.pendingSteer = nil
		}
		line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, "")
		if !ok {
			return t, nil
		}
		collapse := t.collapseToolBuf()
		if collapse != nil {
			return t, tea.Sequence(collapse, tea.Println(line+"\n"))
		}
		return t, tea.Println(line + "\n")

	case agenttypes.EventToolCall:
		if utils.HideToolEvent(ev.ToolName, ev.ToolArgs) {
			return t, nil
		}
		if ev.ToolName != "" {
			if ev.Source != "" {
				t.trackSubagent(ev.Source, toolActivity(ev, t.cwd)).tools++
				return t, nil
			}
			t.activity = "tool: " + ev.ToolName
			t.toolLog = nil
			line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, "")
			if ok {
				t.emitted = true
				if utils.IsSubagentInvoke(ev.ToolName, ev.ToolArgs) {
					t.subCount++
					t.subActive++
				} else {
					t.toolCount++
				}
				t.toolBuf = append(t.toolBuf, line)
			}
			return t, nil
		}

	case agenttypes.EventToolProgress:
		if ev.Source != "" {
			t.trackSubagent(ev.Source, ev.ToolName+": "+oneLine(ev.Text))
			return t, nil
		}
		t.emitted = true
		t.activity = "tool: " + ev.ToolName
		t.toolLog = append(t.toolLog, oneLine(ev.Text))
		if len(t.toolLog) > commandLogLines {
			t.toolLog = t.toolLog[len(t.toolLog)-commandLogLines:]
		}
		return t, nil

	case agenttypes.EventToolSkipped:
		if ev.Source != "" {
			t.trackSubagent(ev.Source, "skipped: "+ev.ToolName)
			return t, nil
		}

	case agenttypes.EventReasoning:
		if ev.Source != "" {
			t.trackSubagent(ev.Source, "✻ "+oneLine(toPureText(ev.Text)))
			return t, nil
		}
		line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, "")
		if !ok {
			return t, nil
		}
		t.emitted = true
		collapse := t.collapseToolBuf()
		if collapse != nil {
			return t, tea.Sequence(collapse, tea.Println(line+"\n"))
		}
		return t, tea.Println(line + "\n")

	case agenttypes.EventToolResult:
		if ev.Source != "" {
			if ev.ToolName == "subagents" {
				t.dropSubagent(ev.Source)
			}
			return t, nil
		}

		if utils.IsSubagentInvoke(ev.ToolName, ev.ToolArgs) {
			t.subActive = max(t.subActive-1, 0)
			if t.subActive == 0 {
				t.subBuf, t.subOrder = nil, nil
			}
		}
		t.activity = ""
		t.toolLog = nil
		return t, nil

	case agenttypes.EventSummaryGenerate:
		t.activity = "summarizing..."

	case agenttypes.EventCompact:
		if ev.Source != "" {
			t.trackSubagent(ev.Source, "compacting...")
			return t, nil
		}
		if ev.Text == "history" {
			t.activity = "compacting history..."
		} else {
			t.activity = "compacting tool history..."
		}
		line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, "")
		if ok {
			t.toolBuf = append(t.toolBuf, line)
		}
		return t, nil

	case agenttypes.EventText:
		if ev.Source == "" {
			t.emitted = true
			collapse := t.collapseToolBuf()
			raw := utils.StripFileMarkers(ev.Text)

			if raw == "" {
				if len(t.tableBuf) == 0 {
					return t, collapse
				}
				cmds := t.flushTableBuf()
				if collapse != nil {
					cmds = append([]tea.Cmd{collapse}, cmds...)
				}
				return t, tea.Sequence(cmds...)
			}

			if len(t.tableBuf) > 0 {
				if strings.Contains(raw, "|") {
					t.tableBuf = append(t.tableBuf, raw)
					if collapse != nil {
						return t, collapse
					}
					return t, nil
				}
				cmds := t.flushTableBuf()
				cmds = append(cmds, t.printStreamLine(renderMarkdown(raw, t.width)))
				if collapse != nil {
					cmds = append([]tea.Cmd{collapse}, cmds...)
				}
				return t, tea.Sequence(cmds...)
			}

			if strings.Contains(raw, "|") {
				t.tableBuf = append(t.tableBuf, raw)
				if collapse != nil {
					return t, collapse
				}
				return t, nil
			}

			if collapse != nil {
				return t, tea.Sequence(collapse, t.printStreamLine(renderMarkdown(raw, t.width)))
			}
			return t, t.printStreamLine(renderMarkdown(raw, t.width))
		}

	case agenttypes.EventTextDone:
		if ev.Source == "" {
			var cmd tea.Cmd
			if len(t.tableBuf) > 0 {
				cmd = tea.Batch(t.flushTableBuf()...)
			}
			t.streaming = false
			return t, cmd
		}
		return t, nil

	case agenttypes.EventDone:
		if ev.Source != "" {
			t.dropSubagent(ev.Source)
			return t, nil
		}
		t.toolLog = nil
		collapse := t.collapseToolBuf()
		t.todos = nil
		t.subBuf, t.subOrder, t.subActive = nil, nil, 0
		if ev.Usage != nil {
			t.tokens = ev.Usage.Input + ev.Usage.Output + ev.Usage.CacheRead + ev.Usage.CacheCreate
			t.lastIn = ev.Usage.Input
			t.lastOut = ev.Usage.Output
			t.lastCacheRead = ev.Usage.CacheRead
			t.lastCacheCreate = ev.Usage.CacheCreate
		}
		finishedAt := formatFinishedAt(time.Now())
		quota := loadQuota(ev.Model)
		if collapse != nil {
			line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, finishedAt)
			if !ok {
				return t, tea.Batch(collapse, quota)
			}
			return t, tea.Batch(tea.Sequence(collapse, tea.Println(line)), quota)
		}
		line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, finishedAt)
		if !ok {
			return t, quota
		}
		return t, tea.Batch(tea.Println(line), quota)

	case agenttypes.EventCanceled:
		if ev.Source != "" {
			t.dropSubagent(ev.Source)
			return t, nil
		}
		t.toolLog = nil
		collapse := t.collapseToolBuf()
		t.todos = nil
		t.subBuf, t.subOrder, t.subActive = nil, nil, 0
		finishedAt := formatFinishedAt(time.Now())
		line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, finishedAt)
		if !ok {
			return t, collapse
		}
		if collapse != nil {
			return t, tea.Sequence(collapse, tea.Println(line))
		}
		return t, tea.Println(line)

	case agenttypes.EventUsageUpdate:
		if ev.Source == "" && ev.Usage != nil {
			t.lastIn = ev.Usage.Input
			t.lastOut = ev.Usage.Output
			t.lastCacheRead = ev.Usage.CacheRead
			t.lastCacheCreate = ev.Usage.CacheCreate
			t.lastContext = ev.ContextTokens
		}
		return t, nil

	}

	line, ok := renderAgentEvent(ev, t.runTarget, t.cwd, t.width, "")
	if !ok {
		return t, nil
	}
	return t, tea.Println(line)
}

func (t *TUI) printStreamLine(line string) tea.Cmd {
	line = wrapText(line, t.width-2)
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

	rendered := renderTables(block, t.width-2)
	rendered = renderMarkdown(rendered, t.width-2)

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

func (t TUI) handleInterrupt() (tea.Model, tea.Cmd) {
	if !t.interruptAt.IsZero() && time.Since(t.interruptAt) <= interruptWindow {
		t.quitting = true
		return t, tea.Sequence(
			notice(msgLog("force quit")+"\n"),
			tea.Quit,
		)
	}

	t.interruptAt = time.Now()
	if t.running && t.cancelExec != nil {
		t.cancelExec(runtime.ErrUserCanceled)
		return t, notice(msgLog("cancelling  ctrl+c again to force quit") + "\n")
	}
	return t, notice(msgLog("ctrl+c again to quit") + "\n")
}

type QuotaLoaded struct {
	model string
	text  string
}

func loadQuota(model string) tea.Cmd {
	return func() tea.Msg {
		return QuotaLoaded{model: model, text: utils.ModelQuota(context.Background(), model)}
	}
}

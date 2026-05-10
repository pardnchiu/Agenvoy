package cli

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	ansiReset  = "\033[0m"
	ansiGray   = "\033[90m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
)

var oneLineReplacer = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

var dollarLinePending atomic.Bool

func removeCommandConfirm() bool {
	return dollarLinePending.Swap(false)
}

func Run(fn func(chan<- agentTypes.Event) error) error {
	start := time.Now()
	ch := make(chan agentTypes.Event, 16)
	var execErr error

	go func() {
		defer close(ch)
		execErr = fn(ch)
	}()

	pendingAgentSelect := false
	var sb strings.Builder
	for ev := range ch {
		// store_secret drives its own stdout interaction (prompt + masked input);
		// any renderer print would race with the prompt and shred the terminal.
		if ev.ToolName == "store_secret" {
			continue
		}

		wasPendingAgentSelect := pendingAgentSelect
		pendingAgentSelect = false

		soruce := strings.TrimSpace(ev.Source)
		if soruce != "" {
			soruce = "  " + soruce + " "
		}

		switch ev.Type {
		case agentTypes.EventSkillResult:
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[*] [%s] Skill: %s", soruce, time.Now().Format("15:04:05"), ev.Text)))

		case agentTypes.EventAgentSelect:
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[~] [%s] Agent: selecting...", soruce, time.Now().Format("15:04:05"))))
			pendingAgentSelect = true

		case agentTypes.EventAgentResult:
			if wasPendingAgentSelect {
				fmt.Print("\033[F\033[2K")
			}
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[*] [%s] Agent: %s", soruce, time.Now().Format("15:04:05"), ev.Text)))

		case agentTypes.EventToolCall:
			if ev.ToolName == "ask_user" {
				break
			}
			if removeCommandConfirm() {
				fmt.Print("\033[F\033[2K")
			}
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[*] [%s] Tool: %s - ", soruce, time.Now().Format("15:04:05"), ev.ToolName)) + ansiGray + utils.FormatTool(ev.ToolName, ev.ToolArgs) + ansiReset)

		case agentTypes.EventToolResult:
			if ev.ToolName == "ask_user" {
				break
			}

		case agentTypes.EventToolSkipped:
			if removeCommandConfirm() {
				fmt.Print("\033[F\033[2K")
			}
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[~] [%s] Skipped: %s - ", soruce, time.Now().Format("15:04:05"), ev.ToolName)) + ansiGray + utils.FormatTool(ev.ToolName, ev.ToolArgs) + ansiReset)

		case agentTypes.EventText:
			text := ev.Text
			if text == "" {
				break
			}
			if ev.Source != "" {
				writeStdoutLine(colorize(ev, fmt.Sprintf("%s%s", soruce, oneLineReplacer.Replace(text))))
				break
			}
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(text)

		case agentTypes.EventTextDone:
			if ev.Source != "" || sb.Len() == 0 {
				break
			}
			full := sb.String()
			sb.Reset()
			if strings.HasPrefix(full, "Agent:") || strings.HasPrefix(full, "Tool:") || strings.HasPrefix(full, "Result:") {
				writeStdoutLine("[*] " + full)
			} else {
				writeStdoutLine("---\n" + full + "\n---")
			}

		case agentTypes.EventExecError:
			writeStderrLine(colorize(ev, fmt.Sprintf("%s[!] [%s] Error: %s - %s", soruce, time.Now().Format("15:04:05"), ev.ToolName, ev.Text)))

		case agentTypes.EventError:
			if ev.Err != nil {
				writeStderrLine(colorize(ev, fmt.Sprintf("%s[!] Error: %v", soruce, ev.Err)))
			}

		case agentTypes.EventSummaryGenerate:
			writeStdoutLine(colorize(ev, soruce+"[*] Generating summary…"))

		case agentTypes.EventDone:
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf(" (%s)", time.Since(start).Round(time.Millisecond)))
			if ev.Model != "" {
				modelDisplay := ev.Model
				if _, after, ok := strings.Cut(ev.Model, "@"); ok {
					modelDisplay = after
				}
				sb.WriteString(fmt.Sprintf(" [%s", modelDisplay))
				if ev.Usage != nil {
					sb.WriteString(fmt.Sprintf(" | in:%d out:%d", ev.Usage.Input, ev.Usage.Output))
				}
				sb.WriteString("]")
			}
			writeStdoutLine(sb.String())
		}
	}

	return execErr
}

func colorize(ev agentTypes.Event, line string) string {
	switch ev.Type {
	case agentTypes.EventExecError, agentTypes.EventError:
		return ansiRed + line + ansiReset
	}
	if ev.Source != "" {
		return ansiGray + line + ansiReset
	}
	return line
}

func writeStdout(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	fmt.Print(text)
}

func writeStdoutLine(text string) {
	writeStdout(text + "\n")
}

func writeStderrLine(text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	fmt.Fprint(os.Stderr, text+"\r\n")
}

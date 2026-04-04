package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

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

func runEvents(_ context.Context, cancel context.CancelFunc, fn func(chan<- agentTypes.Event) error) error {
	start := time.Now()
	ch := make(chan agentTypes.Event, 16)
	var execErr error

	go func() {
		defer close(ch)
		execErr = fn(ch)
	}()

	skillNone := false
	for ev := range ch {
		switch ev.Type {
		case agentTypes.EventSkillSelect:
			writeStdoutLine("[~] Selecting skill…")

		case agentTypes.EventSkillResult:
			if ev.Text == "none" {
				skillNone = true
				writeStdoutLine("[*] Skill: none")
			} else {
				writeStdoutLine(fmt.Sprintf("[*] Skill: %s", ev.Text))
			}

		case agentTypes.EventAgentSelect:
			_ = skillNone
			writeStdoutLine("[~] Selecting agent…")

		case agentTypes.EventAgentResult:
			writeStdoutLine(fmt.Sprintf("[*] Agent: %s", ev.Text))

		case agentTypes.EventToolCall:
			writeStdoutLine(fmt.Sprintf("[*] Tool: %s", ev.ToolName))

		case agentTypes.EventText:
			text := ev.Text
			for {
				start := strings.Index(text, "<summary>")
				end := strings.Index(text, "</summary>")
				if start == -1 || end == -1 || end < start {
					break
				}
				text = strings.TrimSpace(text[:start] + text[end+len("</summary>"):])
			}
			if text == "" {
				break
			}
			if strings.HasPrefix(text, "Agent:") || strings.HasPrefix(text, "Tool:") || strings.HasPrefix(text, "Result:") {
				writeStdoutLine("[*] " + text)
			} else {
				writeStdoutLine("---\n" + text + "\n---")
			}

		case agentTypes.EventToolConfirm:
			prompt := promptui.Select{
				Label:        fmt.Sprintf("Run %s?", ev.ToolName),
				Items:        []string{"Yes", "Skip", "Stop"},
				Size:         3,
				HideSelected: true,
			}
			idx, _, err := prompt.Run()
			if err != nil || idx == 2 {
				writeStdoutLine("[x] User stopped")
				cancel()
				ev.ReplyCh <- false
			} else if idx == 1 {
				writeStdoutLine(fmt.Sprintf("[x] User skipped: %s", ev.ToolName))
				ev.ReplyCh <- false
			} else {
				ev.ReplyCh <- true
			}

		case agentTypes.EventExecError:
			writeStderrLine(fmt.Sprintf("[!] (%s) error: %s", ev.ToolName, ev.Text))

		case agentTypes.EventError:
			if ev.Err != nil {
				writeStderrLine(fmt.Sprintf("[!] Error: %v", ev.Err))
			}

		case agentTypes.EventSummaryGenerate:
			writeStdoutLine("[*] Generating summary…")

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

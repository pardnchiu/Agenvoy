package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func printLog(name, raw string) string {
	if raw == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		return ""
	}
	switch name {
	case "activate_skill":
		if s := pick("skill", "name"); s != "" {
			return s
		}
	case "read_file", "write_file", "patch_file", "list_files", "glob_files", "read_image", "save_page_to_file":
		if s := pick("path", "pattern", "save_to"); s != "" {
			return s
		}
	case "search_web", "fetch_google_rss":
		if q := pick("query", "keyword"); q != "" {
			if tr := pick("time_range", "time"); tr != "" {
				return fmt.Sprintf("%s (%s)", q, tr)
			}
			return q
		}
	case "fetch_yahoo_finance":
		if sym := pick("symbol"); sym != "" {
			if tr := pick("time_range"); tr != "" {
				return fmt.Sprintf("%s (%s)", sym, tr)
			}
			return sym
		}
	case "fetch_page", "fetch_youtube_transcript":
		if s := pick("link", "url"); s != "" {
			return s
		}
	case "remember_error":
		if s := pick("symptom", "cause", "action"); s != "" {
			return s
		}
	case "search_error_memory", "search_conversation_history":
		if s := pick("keyword", "query"); s != "" {
			return s
		}
	case "run_command":
		return printLogCommand(raw)
	}
	return raw
}

func printLogCommand(raw string) string {
	var p struct {
		Argv []string `json:"argv"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil || len(p.Argv) == 0 {
		return raw
	}
	parts := make([]string, len(p.Argv))
	for i, a := range p.Argv {
		if a == "" || strings.ContainsAny(a, " \t\n\"'\\") {
			parts[i] = strconv.Quote(a)
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
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

func runEvents(_ context.Context, cancel context.CancelFunc, fn func(chan<- agentTypes.Event) error) error {
	start := time.Now()
	ch := make(chan agentTypes.Event, 16)
	var execErr error

	go func() {
		defer close(ch)
		execErr = fn(ch)
	}()

	for ev := range ch {
		switch ev.Type {
		case agentTypes.EventSkillResult:
			writeStdoutLine(fmt.Sprintf("[*] Skill: %s", ev.Text))

		case agentTypes.EventAgentSelect:
			writeStdoutLine("[~] Selecting agent…")

		case agentTypes.EventAgentResult:
			writeStdoutLine(fmt.Sprintf("[*] Agent: %s", ev.Text))

		case agentTypes.EventToolCall:
			writeStdoutLine(fmt.Sprintf("[*] [%s] Tool: %s - %s", time.Now().Format("15:04:05"), ev.ToolName, printLog(ev.ToolName, ev.ToolArgs)))

		case agentTypes.EventToolSkipped:
			writeStdoutLine(fmt.Sprintf("[~] [%s] Tool skipped: %s", time.Now().Format("15:04:05"), ev.ToolName))

		case agentTypes.EventText:
			text := ev.Text
			if text == "" {
				break
			}
			if strings.HasPrefix(text, "Agent:") || strings.HasPrefix(text, "Tool:") || strings.HasPrefix(text, "Result:") {
				writeStdoutLine("[*] " + text)
			} else {
				writeStdoutLine("---\n" + text + "\n---")
			}

		case agentTypes.EventToolConfirm:
			if ev.ToolName == "run_command" {
				writeStdoutLine(fmt.Sprintf("[$] %s", printLogCommand(ev.ToolArgs)))
			}
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
			writeStderrLine(fmt.Sprintf("[!] [%s] Error: %s - %s", time.Now().Format("15:04:05"), ev.ToolName, ev.Text))

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

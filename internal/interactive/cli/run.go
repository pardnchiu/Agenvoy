package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
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
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[*] [%s] Tool: %s - ", soruce, time.Now().Format("15:04:05"), ev.ToolName)) + ansiGray + printLog(ev.ToolName, ev.ToolArgs) + ansiReset)

		case agentTypes.EventToolResult:
			if ev.ToolName == "ask_user" {
				break
			}

		case agentTypes.EventToolSkipped:
			if removeCommandConfirm() {
				fmt.Print("\033[F\033[2K")
			}
			writeStdoutLine(colorize(ev, fmt.Sprintf("%s[~] [%s] Skipped: %s - ", soruce, time.Now().Format("15:04:05"), ev.ToolName)) + ansiGray + printLog(ev.ToolName, ev.ToolArgs) + ansiReset)

		case agentTypes.EventText:
			text := ev.Text
			if text == "" {
				break
			}
			if ev.Source != "" {
				writeStdoutLine(colorize(ev, fmt.Sprintf("%s%s", soruce, oneLineReplacer.Replace(text))))
				break
			}
			if strings.HasPrefix(text, "Agent:") || strings.HasPrefix(text, "Tool:") || strings.HasPrefix(text, "Result:") {
				writeStdoutLine("[*] " + text)
			} else {
				writeStdoutLine("---\n" + text + "\n---")
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
	case "invoke_subagent":
		return printLogSubagent(m, pick)
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
	case "calculate":
		if s := pick("expression"); s != "" {
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

func printLogSubagent(_ map[string]any, pick func(...string) string) string {
	label := pick("name", "session_id")
	if label == "" {
		label = "subagent"
	}
	if model := pick("model"); model != "" {
		label = fmt.Sprintf("%s (%s)", label, model)
	}
	task := pick("task")
	if task == "" {
		return label
	}
	return fmt.Sprintf("%s: %s", label, oneLineReplacer.Replace(task))
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

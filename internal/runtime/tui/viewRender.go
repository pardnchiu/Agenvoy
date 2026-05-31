package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var projectVersion = "dev"

var (
	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colSystem).
			Padding(0, 2)

	textAreaStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(colHint).
			Padding(0, 1)

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colWarn).
			Padding(0, 1)
)

func headerBlock(cwd, daemon, http, discord, telegram string) string {
	logo := textStyle.Bold(true).Render("✻ Agenvoy ") + hintStyle.Render(projectVersion)
	cwdStyle := textStyle
	displayCwd := cwd
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		switch {
		case cwd == home:
			cwdStyle = hintStyle
			displayCwd = "~"
		case strings.HasPrefix(cwd, home+"/"):
			displayCwd = "~" + cwd[len(home):]
		}
	}
	body := strings.Join([]string{
		logo,
		"",
		textStyle.Render("/         ") + hintStyle.Render("list commands"),
		textStyle.Render("/switch   ") + hintStyle.Render("change current session"),
		textStyle.Render("/bot      ") + hintStyle.Render("edit bot persona"),
		textStyle.Render("/mode     ") + hintStyle.Render("switch mode (cli / web)"),
		"",
		cwdStyle.Render("cwd:      " + displayCwd),
		daemon,
		http,
		discord,
		telegram,
	}, "\n")
	return headerStyle.Render(body)
}

func messageBlock(str string) string {
	var sb strings.Builder
	for i, line := range strings.Split(str, "\n") {
		if i > 0 {
			sb.WriteString("\n  ")
		} else {
			sb.WriteString(hintStyle.Render("❯ "))
		}
		sb.WriteString(userStyle.Render(line))
	}
	sb.WriteString("\n")
	return sb.String()
}

func messageRow(text, subagent string) string {
	prefix := systemStyle.Render("⏺ ")
	if strings.TrimSpace(subagent) != "" {
		prefix = warnStyle.Render("⏺ [" + subagent + "] ")
	}
	indent := "  "

	var sb strings.Builder
	first := true
	for line := range strings.SplitSeq(text, "\n") {
		if first {
			sb.WriteString(prefix)
			sb.WriteString(line)
			first = false
			continue
		}
		sb.WriteByte('\n')
		sb.WriteString(indent)
		sb.WriteString(line)
	}
	return sb.String()
}

func renderAgentEvent(ev agentTypes.Event, sessionLabel, cwd string) (string, bool) {
	src := strings.TrimSpace(ev.Source)
	srcPrefix := ""
	if src != "" {
		srcPrefix = "[" + src + "] "
	}

	switch ev.Type {
	case agentTypes.EventSkillResult:
		return hintStyle.Render("⏵ " + srcPrefix + "Skill(" + ev.Text + ")"), true

	case agentTypes.EventAgentSelect:
		if ev.Source == "" {
			return "", false
		}
		return hintStyle.Render("  ⎿ " + srcPrefix + "selecting agent…"), true

	case agentTypes.EventAgentResult:
		if ev.Source == "" {
			return "", false
		}
		str := strings.TrimSpace(ev.Text)
		if str == "" {
			return "", false
		}
		return hintStyle.Render("  ⎿ " + srcPrefix + "agent: " + str), true

	case agentTypes.EventToolCall:
		if ev.ToolName == "ask_user" || ev.ToolName == "store_secret" {
			return "", false
		}
		bullet := "⏵"
		if ev.Source != "" {
			bullet = "  ⎿"
		}
		return buildToolLine(bullet, ev.Source, ev.ToolName, ev.ToolArgs, cwd), true

	case agentTypes.EventToolSkipped:
		line := "  ⎿ " + srcPrefix + "skipped: " + ev.ToolName
		if arg := printLog(ev.ToolName, ev.ToolArgs, cwd); arg != "" {
			line += "(" + truncate(arg, 120) + ")"
		}
		return hintStyle.Render(line), true

	case agentTypes.EventText:
		str := ev.Text
		if str == "" {
			return "", false
		}
		if ev.Source != "" {
			return hintStyle.Render("  ⎿ " + srcPrefix + oneLine(str)), true
		}
		return messageRow(str, sessionLabel), true

	case agentTypes.EventExecError:
		return errorStyle.Render("  ⎿ " + srcPrefix + "error: " + ev.ToolName + " — " + ev.Text), true

	case agentTypes.EventError:
		if ev.Err == nil {
			return "", false
		}
		return errorStyle.Render("  ⎿ " + srcPrefix + fmt.Sprintf("error: %v", ev.Err)), true

	case agentTypes.EventSummaryGenerate:
		return hintStyle.Render("⏵ " + srcPrefix + "summarizing…"), true

	case agentTypes.EventDone:
		footer := utils.FormatEventFooter(ev.Duration, ev.Model, ev.Usage)
		if sessionLabel != "" {
			if footer != "" {
				footer = footer + " · [" + sessionLabel + "]"
			} else {
				footer = "[" + sessionLabel + "]"
			}
		}
		if footer == "" {
			return "", false
		}
		return hintStyle.Render("  ⎿ "+footer) + "\n", true
	}

	return "", false
}

func buildToolLine(bullet, source, name, args, cwd string) string {
	src := strings.TrimSpace(source)
	srcPrefix := ""
	if src != "" {
		srcPrefix = "[" + src + "] "
	}
	line := bullet + " " + srcPrefix + name
	if arg := printLog(name, args, cwd); arg != "" {
		line += "(" + truncate(arg, 120) + ")"
	}
	style := hintStyle
	if name == "invoke_subagent" {
		style = lipgloss.NewStyle().Foreground(colOk)
	}
	return style.Render(line)
}

func oneLine(s string) string {
	r := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	return r.Replace(s)
}

func isCwd(dir, cwd string) bool {
	d := strings.TrimRight(strings.TrimSpace(dir), "/")
	if d == "." || d == "./" || d == "" {
		return true
	}
	c := strings.TrimRight(strings.TrimSpace(cwd), "/")
	return c != "" && d == c
}

func printLog(name, raw, cwd string) string {
	if raw == "" {
		return ""
	}
	var dic map[string]any
	if err := json.Unmarshal([]byte(raw), &dic); err != nil {
		return raw
	}
	if len(dic) == 0 {
		return ""
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := dic[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		return ""
	}
	switch name {
	case "invoke_subagent":
		label := pick("name", "session_id")
		if label == "" {
			label = "subagent"
		}
		if model := pick("model"); model != "" {
			label = fmt.Sprintf("%s (%s)", label, model)
		}
		if task := pick("task"); task != "" {
			return fmt.Sprintf("%s: %s", label, oneLine(task))
		}
		return label

	case "activate_skill":
		if s := pick("skill", "name"); s != "" {
			return s
		}

	case "list_files":
		dir := pick("dir", "path")
		if dir == "" {
			break
		}
		if r, ok := dic["recursive"].(bool); ok && r {
			return dir + " (recursive)"
		}
		return dir

	case "read_file", "write_file", "patch_file", "glob_files", "read_image", "save_page_to_file":
		if s := pick("path", "pattern", "save_to"); s != "" {
			return s
		}

	case "update_page":
		return ""

	case "search_files":
		dir := strings.TrimSpace(pick("dir"))
		if dir == "" {
			dir = "."
		}
		if isCwd(dir, cwd) {
			dir = "./"
		}
		loc := dir
		if fp := strings.TrimSpace(pick("file_pattern")); fp != "" {
			loc = strings.TrimRight(dir, "/") + "/" + fp
		}
		if pat := pick("pattern"); pat != "" {
			return loc + " [" + pat + "]"
		}
		return loc

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

	case "add_task", "add_cron", "patch_task", "patch_cron":
		skill := pick("skill_name")
		t := pick("time")
		if skill != "" && t != "" {
			return fmt.Sprintf("%s %s", t, skill)
		}
		if skill != "" {
			return skill
		}

	case "remove_task", "remove_cron":
		if skill := pick("skill_name"); skill != "" {
			return skill
		}

	case "run_command":
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
	return raw
}

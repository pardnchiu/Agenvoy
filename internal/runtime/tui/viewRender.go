package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var (
	mdBoldRe  = regexp.MustCompile(`\*\*(.+?)\*\*`)
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
)

func toPureText(s string) string {
	s = mdBoldRe.ReplaceAllString(s, "$1")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}

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

func headerBlock(daemon, http, discord, telegram string) string {
	logo := whiteStyle.Bold(true).Render("Agenvoy ") + hintStyle.Render(projectVersion)

	const leftCol = 14
	const gap = "   "
	padLeft := func(s string) string {
		w := lipgloss.Width(s)
		if w >= leftCol {
			return s
		}
		return s + strings.Repeat(" ", leftCol-w)
	}

	body := strings.Join([]string{
		logo,
		hintStyle.Render("Make AI actually work for you"),
		hintStyle.Render("Your productivity infrastructure"),
		"",
		padLeft(daemon) + gap + discord,
		padLeft(http) + gap + telegram,
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
		if arg := utils.FormatToolArgs(ev.ToolName, ev.ToolArgs, cwd); arg != "" {
			line += "(" + go_pkg_utils.TruncateString(arg, 128) + ")"
		}
		return hintStyle.Render(line), true

	case agentTypes.EventText:
		str := toPureText(ev.Text)
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

var (
	diffOldStyle = lipgloss.NewStyle().Foreground(colError)
	diffNewStyle = lipgloss.NewStyle().Foreground(colOk)
)

func buildToolLine(bullet, source, name, args, cwd string) string {
	src := strings.TrimSpace(source)
	srcPrefix := ""
	if src != "" {
		srcPrefix = "[" + src + "] "
	}
	line := bullet + " " + srcPrefix + utils.ToolName(name)
	if arg := utils.FormatToolArgs(name, args, cwd); arg != "" {
		line += "(" + go_pkg_utils.TruncateString(arg, 128) + ")"
	}
	style := hintStyle
	if name == "invoke_subagent" {
		style = lipgloss.NewStyle().Foreground(colOk)
	}
	header := style.Render(line)

	switch name {
	case "patch_file", "patch_tool", "patch_skill":
		oldLines, newLines := utils.FormatPatchDiff(args)
		if len(oldLines) == 0 && len(newLines) == 0 {
			return header
		}
		var sb strings.Builder
		sb.WriteString(header)
		for _, l := range oldLines {
			sb.WriteByte('\n')
			sb.WriteString(diffOldStyle.Render("  - " + go_pkg_utils.TruncateString(l, 120)))
		}
		for _, l := range newLines {
			sb.WriteByte('\n')
			sb.WriteString(diffNewStyle.Render("  + " + go_pkg_utils.TruncateString(l, 120)))
		}
		return sb.String()

	case "write_file":
		lines := utils.FormatWriteDiff(args)
		if len(lines) == 0 {
			return header
		}
		var sb strings.Builder
		sb.WriteString(header)
		for _, l := range lines {
			sb.WriteByte('\n')
			sb.WriteString(diffNewStyle.Render("  + " + go_pkg_utils.TruncateString(l, 120)))
		}
		return sb.String()
	}

	return header
}

func oneLine(s string) string {
	r := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	return r.Replace(s)
}

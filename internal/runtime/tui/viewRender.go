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
	mdHeadingRe    = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)`)
	mdBoldRe       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdItalicRe     = regexp.MustCompile(`\*([^\s*](?:[^*]*[^\s*])?)\*`)
	mdBlockquoteRe = regexp.MustCompile(`(?m)^>\s?(.*)`)
	htmlTagRe      = regexp.MustCompile(`<[^>]*>`)
)

func toPureText(s string) string {
	s = mdBoldRe.ReplaceAllString(s, "$1")
	s = htmlTagRe.ReplaceAllString(s, "")
	return s
}

func renderMarkdown(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = renderTables(s)
	s = mdBlockquoteRe.ReplaceAllStringFunc(s, func(match string) string {
		m := mdBlockquoteRe.FindStringSubmatch(match)
		content := mdBoldRe.ReplaceAllString(m[1], "$1")
		content = mdItalicRe.ReplaceAllString(content, "$1")
		return systemStyle.Render("▎ " + content)
	})
	s = mdBoldRe.ReplaceAllStringFunc(s, func(match string) string {
		return userStyle.Bold(true).Render(match[2 : len(match)-2])
	})
	s = mdItalicRe.ReplaceAllString(s, "$1")
	s = mdHeadingRe.ReplaceAllStringFunc(s, func(match string) string {
		m := mdHeadingRe.FindStringSubmatch(match)
		return okayStyle.Bold(true).Render(m[2])
	})
	return s
}

func isTableSep(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "|") {
		return false
	}
	dashes := 0
	for _, c := range trimmed {
		switch c {
		case '-':
			dashes++
		case '|', ':', ' ':
		default:
			return false
		}
	}
	return dashes >= 3
}

func parseTableCells(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func cleanTableCell(s string) string {
	s = mdBoldRe.ReplaceAllString(s, "$1")
	s = mdItalicRe.ReplaceAllString(s, "$1")
	return s
}

func renderTables(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		if i+1 < len(lines) && strings.Contains(lines[i], "|") && isTableSep(lines[i+1]) {
			end := i + 2
			for end < len(lines) && strings.Contains(lines[end], "|") && !isTableSep(lines[end]) {
				end++
			}
			header := parseTableCells(lines[i])
			var rows [][]string
			for j := i + 2; j < end; j++ {
				rows = append(rows, parseTableCells(lines[j]))
			}
			out = append(out, buildTable(header, rows))
			i = end
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n")
}

func buildTable(header []string, rows [][]string) string {
	numCols := len(header)
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	for len(header) < numCols {
		header = append(header, "")
	}
	for i := range rows {
		for len(rows[i]) < numCols {
			rows[i] = append(rows[i], "")
		}
	}

	for i := range header {
		header[i] = cleanTableCell(header[i])
	}
	for i := range rows {
		for j := range rows[i] {
			rows[i][j] = cleanTableCell(rows[i][j])
		}
	}

	widths := make([]int, numCols)
	for i, h := range header {
		if w := lipgloss.Width(h); w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := lipgloss.Width(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	b := func(s string) string { return hintStyle.Render(s) }

	var sb strings.Builder

	hLine := func(left, mid, right string) {
		sb.WriteString(b(left))
		for i, w := range widths {
			sb.WriteString(b(strings.Repeat("─", w+2)))
			if i < numCols-1 {
				sb.WriteString(b(mid))
			}
		}
		sb.WriteString(b(right))
	}

	writeRow := func(cells []string, bold bool) {
		for i, cell := range cells {
			sb.WriteString(b("│"))
			pad := max(widths[i]-lipgloss.Width(cell), 0)
			if bold {
				left := pad / 2
				right := pad - left
				sb.WriteString(strings.Repeat(" ", left+1))
				sb.WriteString(whiteStyle.Bold(true).Render(cell))
				sb.WriteString(strings.Repeat(" ", right+1))
			} else {
				sb.WriteByte(' ')
				sb.WriteString(cell)
				sb.WriteString(strings.Repeat(" ", pad+1))
			}
		}
		sb.WriteString(b("│"))
	}

	hLine("┌", "┬", "┐")
	sb.WriteByte('\n')
	writeRow(header, true)
	sb.WriteByte('\n')
	hLine("├", "┼", "┤")

	for i, row := range rows {
		sb.WriteByte('\n')
		writeRow(row, false)
		if i < len(rows)-1 {
			sb.WriteByte('\n')
			hLine("├", "┼", "┤")
		}
	}

	sb.WriteByte('\n')
	hLine("└", "┴", "┘")

	return sb.String()
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
		if ev.ToolName == "ask_user" || ev.ToolName == "store_secret" ||
			ev.ToolName == "list_recent_tool_call" || ev.ToolName == "read_tool_call" {
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
		if ev.Source != "" {
			str := toPureText(ev.Text)
			if str == "" {
				return "", false
			}
			return hintStyle.Render("  ⎿ " + srcPrefix + oneLine(str)), true
		}
		str := renderMarkdown(ev.Text)
		if str == "" {
			return "", false
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

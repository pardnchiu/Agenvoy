package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type CmdSelector struct {
	items  []CmdSelectorItem
	cursor int
}

type CmdSelectorItem struct {
	label   string
	desc    string
	isSkill bool
}

type Command struct {
	name string
	desc string
}

var commands = []Command{
	{"model", "add / remove provider · pick session model"},
	{"planner", "pick / set planner model from registry"},
	{"reasoning", "set reasoning depth · global (planner) / session"},
	{"switch", "switch / change current session via picker"},
	{"new", "create / add new session · name conflict-checked"},
	{"bot", "edit / rename current session · name / description (persona)"},
	{"discord", "enable / disable Discord bot · gateway validated on enable"},
	{"update", "update / upgrade · fetch latest release · rebuild · quit TUI"},
	{"mode", "switch / change rendering · TUI (cli) or browser (web)"},
	{"clear", "clear visible transcript / history · memory untouched"},
	{"exit", "exit / quit TUI · daemon keeps running"},
}

func (t TUI) refreshCmdSelector() TUI {
	query, ok := queryCmdSelector(t.textarea.Value())
	if !ok {
		t.selector = nil
		return t
	}

	items := getCmdSelectorItems(query)
	if len(items) == 0 {
		t.selector = nil
		return t
	}

	cursor := 0
	if t.selector != nil && t.selector.cursor < len(items) {
		cursor = t.selector.cursor
	}
	t.selector = &CmdSelector{
		items:  items,
		cursor: cursor,
	}
	return t
}

func queryCmdSelector(content string) (query string, ok bool) {
	trimmed := strings.TrimLeft(content, " \t")
	if !strings.HasPrefix(trimmed, "/") {
		return "", false
	}

	rest := trimmed[1:]
	if strings.ContainsAny(rest, " \t\n\r") {
		return "", false
	}
	return rest, true
}

func getCmdSelectorItems(query string) []CmdSelectorItem {
	query = strings.ToLower(query)
	var cmdNameItems, cmdDescItems, skillItems []CmdSelectorItem

	for _, c := range commands {
		item := CmdSelectorItem{
			label: "/" + c.name,
			desc:  c.desc,
		}
		switch {
		case query == "" || strings.Contains(c.name, query):
			cmdNameItems = append(cmdNameItems, item)
		case strings.Contains(strings.ToLower(c.desc), query):
			cmdDescItems = append(cmdDescItems, item)
		}
	}

	if scanner := host.Scanner(); scanner != nil {
		for _, name := range scanner.List() {
			if name == "" {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(name), query) {
				continue
			}

			source := ""
			description := ""
			if scanner.Skills != nil {
				if sk := scanner.Skills.ByName[name]; sk != nil {
					source = skillSource(sk.AbsPath)
					description = sk.Description
				}
			}
			desc := description
			if source != "" {
				if description != "" {
					desc = "(" + source + ") " + description
				} else {
					desc = "(" + source + ")"
				}
			}
			if desc == "" {
				desc = "skill"
			}
			skillItems = append(skillItems, CmdSelectorItem{
				label:   "/" + name,
				desc:    desc,
				isSkill: true,
			})
		}
	}

	byLabel := func(s []CmdSelectorItem) func(i, j int) bool {
		return func(i, j int) bool { return s[i].label < s[j].label }
	}
	sort.SliceStable(cmdNameItems, byLabel(cmdNameItems))
	sort.SliceStable(cmdDescItems, byLabel(cmdDescItems))
	sort.SliceStable(skillItems, byLabel(skillItems))

	items := make([]CmdSelectorItem, 0, len(cmdNameItems)+len(cmdDescItems)+len(skillItems))
	items = append(items, cmdNameItems...)
	items = append(items, cmdDescItems...)
	items = append(items, skillItems...)
	return items
}

func skillSource(path string) string {
	if path == "" {
		return ""
	}
	switch {
	case filesystem.SystemSkillsDir != "" && strings.HasPrefix(path, filesystem.SystemSkillsDir+"/"):
		return "system"
	case filesystem.SkillsDir != "" && strings.HasPrefix(path, filesystem.SkillsDir+"/"):
		return "agenvoy"
	case strings.Contains(path, "/.claude/skills/"):
		return "claude"
	case strings.Contains(path, "/.opencode/skills/"):
		return "opencode"
	case strings.Contains(path, "/.openai/skills/"):
		return "openai"
	case strings.Contains(path, "/.codex/skills/"):
		return "codex"
	case strings.Contains(path, "/.skills/"):
		return "local"
	case strings.HasPrefix(path, "/mnt/skills/"):
		rest := strings.TrimPrefix(path, "/mnt/skills/")
		if i := strings.IndexByte(rest, '/'); i > 0 {
			return "mnt-" + rest[:i]
		}
	}
	return ""
}

func (t TUI) selectCommand() TUI {
	if t.selector == nil || t.selector.cursor >= len(t.selector.items) {
		return t
	}
	chosen := t.selector.items[t.selector.cursor]
	t.textarea.SetValue(chosen.label + " ")
	t.textarea.CursorEnd()
	t.selector = nil
	return t
}

const cmdSelectorMaxVisible = 8

func renderCmdSelector(p *CmdSelector) string {
	if p == nil || len(p.items) == 0 {
		return ""
	}
	total := len(p.items)
	start, end := windowRange(p.cursor, total, cmdSelectorMaxVisible)

	maxLabel := 0
	for _, it := range p.items[start:end] {
		if w := lipgloss.Width(it.label); w > maxLabel {
			maxLabel = w
		}
	}
	var lines []string
	for i := start; i < end; i++ {
		it := p.items[i]
		marker := "  "
		labelStyle := textStyle
		if i == p.cursor {
			marker = systemStyle.Render("> ")
			labelStyle = systemStyle
			if it.isSkill {
				labelStyle = skillStyle
			}
		}
		pad := strings.Repeat(" ", maxLabel-lipgloss.Width(it.label))
		line := marker + labelStyle.Render(it.label) + pad
		if it.desc != "" {
			line += "  " + hintStyle.Render(it.desc)
		}
		lines = append(lines, line)
	}
	if total > cmdSelectorMaxVisible {
		lines = append(lines, hintStyle.Render(fmt.Sprintf("  %d/%d", p.cursor+1, total)))
	}
	return strings.Join(lines, "\n")
}

func windowRange(cursor, total, size int) (start, end int) {
	if total <= size {
		return 0, total
	}
	start = cursor - size/2
	start = max(start, 0)
	end = start + size
	if end > total {
		end = total
		start = end - size
	}
	return start, end
}

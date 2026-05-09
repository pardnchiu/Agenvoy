package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type CmdSelector struct {
	items  []CmdSelectorItem
	cursor int
}

type CmdSelectorItem struct {
	label  string
	desc   string
	insert string
}

type Command struct {
	name string
	desc string
}

var commands = []Command{
	{"model-add", "add a model (opens interactive flow)"},
	{"model-remove", "remove a configured model"},
	{"planner", "set the planner model"},
	{"reasoning", "set planner reasoning level"},
	{"session-model", "set current session model + reasoning"},
	{"switch", "change current session"},
	{"new", "create and switch to a new session"},
	{"bot", "edit current session bot.md in $EDITOR"},
	{"discord-enable", "enable discord bot (token + guild)"},
	{"discord-disable", "disable discord bot (token preserved)"},
	{"update", "update agen to latest release (exits TUI)"},
	{"clear", "clear conversation"},
	{"exit", "quit"},
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
	var items []CmdSelectorItem

	for _, c := range commands {
		if query == "" || strings.Contains(c.name, query) {
			items = append(items, CmdSelectorItem{
				label:  "/" + c.name,
				desc:   c.desc,
				insert: "/" + c.name + " ",
			})
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

			desc := "skill"
			if scanner.Skills != nil {
				if sk := scanner.Skills.ByName[name]; sk != nil {
					if src := skillSource(sk.AbsPath); src != "" {
						desc = "skill (" + src + ")"
					}
				}
			}
			items = append(items, CmdSelectorItem{
				label:  "/" + name,
				desc:   desc,
				insert: "/" + name + " ",
			})
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].label < items[j].label
	})
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
	t.textarea.SetValue(chosen.insert)
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
		if w := len([]rune(it.label)); w > maxLabel {
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
		}
		pad := strings.Repeat(" ", maxLabel-len([]rune(it.label)))
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

package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type CmdSelector struct {
	items  []CmdSelectorItem
	cursor int
}

type CmdSelectorItem struct {
	label       string
	desc        string
	descStyled  string
	isSkill     bool
	isScheduler bool
	isAllow     bool
}

type Command struct {
	name string
	desc string
}

var commands = []Command{
	{"model", "add / remove provider  pick session / dispatch / summary model  decision / image / stt / tts"},
	{"mcp", "list MCP servers  add  per-server login, reconnect, tools, remove"},
	{"session", "switch / delete sessions  edit current session name / id / role  /session name|id|role opens one directly"},
	{"new", "create / add new session"},
	{"skill", "always-allow skills (skip permission)  install / remove official skills from github.com/agenvoy/skill-<name>"},
	{"compact", "remove redundant / meaningless exchanges from history via LLM analysis  confirm required"},
	{"reset", "reset / refresh current session  double-confirm  summary regen first then drop history + task history + action.log"},
	{"role", "list / add / edit role  title + description"},
	{"channel", "enable / disable Telegram, Discord or LINE bot  token validated on enable"},
	{"config", "search settings  startup on login  reply language  output dir"},
	{"schedule", "recurring (cron) and one-shot (task) in one list  enter fires it now  d deletes it  add / edit by asking the agent"},
	{"update", "update / upgrade  fetch latest release  rebuild  quit TUI"},
	{"resume", "reload visible transcript  last 100 entries from action.log"},
	{"log", "follow / tail raw daemon.log via $PAGER (less +F)  ctrl-c stops following"},
	{"usage", "per-model token usage  session above, global below  24h / 7d / 28d"},
	{"pending", "list / resume interrupted tasks  error recovery  ask_user resume"},
	{"key", "update / rotate keychain value  pick from recorded keys"},
	{"clear", "clear visible transcript / history  memory untouched"},
	{"exit", "exit / quit TUI  daemon keeps running"},
}

func (t TUI) refreshCmdSelector() TUI {
	query, ok := queryCmdSelector(t.textarea.Value())
	if !ok || t.running {
		t.selector = nil
		return t
	}

	if t.selector == nil {
		if scanner := agents.Scanner(); scanner != nil {
			scanner.Scan()
		}
	}

	items := getCmdSelectorItems(query, strings.TrimSpace(t.currentSessionID))
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

func isDangerCommand(name string) bool {
	return strings.HasPrefix(name, "allow-")
}

func getCmdSelectorItems(query, sessionID string) []CmdSelectorItem {
	query = strings.ToLower(query)

	var (
		prefixCmd   []CmdSelectorItem
		prefixSk    []CmdSelectorItem
		prefixSched []CmdSelectorItem
		descCmd     []CmdSelectorItem
		descSk      []CmdSelectorItem
		descSched   []CmdSelectorItem
		dangerCmd   []CmdSelectorItem
		dangerSk    []CmdSelectorItem
	)

	for _, c := range commands {
		item := CmdSelectorItem{
			label: "/" + c.name,
			desc:  c.desc,
		}
		if isDangerCommand(c.name) {
			item.isAllow = true
			if strings.HasPrefix(c.desc, "!(") {
				if i := strings.IndexByte(c.desc, ')'); i >= 0 {
					item.descStyled = errorStyle.Render(c.desc[:i+1]) + hintStyle.Render(c.desc[i+1:])
				}
			}
			if item.descStyled == "" {
				item.descStyled = errorStyle.Render(c.desc)
			}
		}
		switch {
		case item.isAllow && (query == "" || strings.Contains(c.name, query) || strings.Contains(strings.ToLower(c.desc), query)):
			dangerCmd = append(dangerCmd, item)
		case query == "" || strings.HasPrefix(c.name, query) || strings.Contains(c.name, query):
			prefixCmd = append(prefixCmd, item)
		case strings.Contains(strings.ToLower(c.desc), query):
			descCmd = append(descCmd, item)
		}
	}

	if scanner := agents.Scanner(); scanner != nil {
		for _, name := range scanner.List() {
			if name == "" {
				continue
			}

			source := ""
			description := ""
			if scanner.Skills != nil {
				if sk := scanner.Skills.ByName[name]; sk != nil {
					source = skillSource(sk.AbsPath)
					description = strings.Join(strings.Fields(sk.Description), " ")
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
			item := CmdSelectorItem{
				label:   "/" + name,
				desc:    desc,
				isSkill: true,
			}

			lowerName := strings.ToLower(name)
			switch {
			case query == "" || strings.HasPrefix(lowerName, query) || strings.Contains(lowerName, query):
				prefixSk = append(prefixSk, item)
			case strings.Contains(strings.ToLower(description), query):
				descSk = append(descSk, item)
			}
		}
	}

	cronByName := make(map[string]string)
	skillBoundToSession := make(map[string]bool)
	if crons, err := runtime.LoadCrons(); err == nil {
		for _, c := range crons {
			if _, exists := cronByName[c.Skill]; !exists {
				cronByName[c.Skill] = c.Expression
			}
			if strings.TrimSpace(c.SessionID) == sessionID {
				skillBoundToSession[c.Skill] = true
			}
		}
	}
	if tasks, err := runtime.LoadTasks(); err == nil {
		for _, t := range tasks {
			if strings.TrimSpace(t.SessionID) == sessionID {
				skillBoundToSession[t.Skill] = true
			}
		}
	}
	if dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.ScheduleSkillsDir); err == nil {
		for _, d := range dirs {
			name := d.Name
			if name == "" || name[0] == '.' {
				continue
			}
			if !skillBoundToSession[name] {
				continue
			}
			label := "/sched-" + name
			desc := "scheduler skill"
			if expr := cronByName[name]; expr != "" {
				desc = "(" + expr + ") scheduler skill"
			}
			item := CmdSelectorItem{
				label:       label,
				desc:        desc,
				isScheduler: true,
			}

			lowerLabel := strings.ToLower(strings.TrimPrefix(label, "/"))
			switch {
			case query == "" || strings.HasPrefix(lowerLabel, query) || strings.Contains(lowerLabel, query):
				prefixSched = append(prefixSched, item)
			case strings.Contains(strings.ToLower(desc), query):
				descSched = append(descSched, item)
			}
		}
	}

	byLabel := func(s []CmdSelectorItem) func(i, j int) bool {
		return func(i, j int) bool { return s[i].label < s[j].label }
	}
	sort.SliceStable(prefixCmd, func(i, j int) bool {
		pi := strings.HasPrefix(strings.TrimPrefix(prefixCmd[i].label, "/"), query)
		pj := strings.HasPrefix(strings.TrimPrefix(prefixCmd[j].label, "/"), query)
		if pi != pj {
			return pi
		}
		return prefixCmd[i].label < prefixCmd[j].label
	})
	sort.SliceStable(prefixSk, func(i, j int) bool {
		pi := strings.HasPrefix(strings.ToLower(strings.TrimPrefix(prefixSk[i].label, "/")), query)
		pj := strings.HasPrefix(strings.ToLower(strings.TrimPrefix(prefixSk[j].label, "/")), query)
		if pi != pj {
			return pi
		}
		return prefixSk[i].label < prefixSk[j].label
	})
	sort.SliceStable(prefixSched, byLabel(prefixSched))
	sort.SliceStable(descCmd, byLabel(descCmd))
	sort.SliceStable(descSk, byLabel(descSk))
	sort.SliceStable(descSched, byLabel(descSched))
	sort.SliceStable(dangerCmd, byLabel(dangerCmd))
	sort.SliceStable(dangerSk, byLabel(dangerSk))

	items := make([]CmdSelectorItem, 0, len(prefixCmd)+len(prefixSk)+len(prefixSched)+len(descCmd)+len(descSk)+len(descSched)+len(dangerCmd)+len(dangerSk))
	items = append(items, prefixCmd...)
	items = append(items, prefixSk...)
	items = append(items, prefixSched...)
	items = append(items, descCmd...)
	items = append(items, descSk...)
	items = append(items, descSched...)
	items = append(items, dangerCmd...)
	items = append(items, dangerSk...)
	return items
}

func skillSource(path string) string {
	return runtime.SkillSource(path)
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

const cmdSelectorMaxVisible = 12

func renderCmdSelector(p *CmdSelector) string {
	if p == nil || len(p.items) == 0 {
		return ""
	}
	total := len(p.items)
	start, end := windowRange(p.cursor, total, cmdSelectorMaxVisible)

	const minLabelWidth = 16
	maxLabel := minLabelWidth
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
		if it.isAllow {
			labelStyle = errorStyle
		}
		if i == p.cursor {
			marker = systemStyle.Render("⏵ ")
			labelStyle = systemStyle
			switch {
			case it.isScheduler:
				labelStyle = warnStyle
			case it.isSkill:
				labelStyle = skillStyle
			case it.isAllow:
				labelStyle = errorStyle
			}
		}
		pad := strings.Repeat(" ", maxLabel-lipgloss.Width(it.label))
		line := marker + labelStyle.Render(it.label) + pad
		switch {
		case it.descStyled != "":
			line += "  " + it.descStyled
		case it.desc != "":
			line += "  " + hintStyle.Render(it.desc)
		}
		lines = append(lines, line)
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

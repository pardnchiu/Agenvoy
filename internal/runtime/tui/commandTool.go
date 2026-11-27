package tui

import (
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/configs"
	agentsexecallowtool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	runtimetooladapter "github.com/pardnchiu/agenvoy/internal/runtime/toolAdapter"
)

func (t TUI) commandTool() (TUI, tea.Cmd, bool) {
	next, cmd := t.openLoading("/tool", toolPopup)
	return next, cmd, true
}

func toolPopup() *Popup {
	popup := &Popup{
		kind:        popupSingleSelect,
		title:       "/tool",
		enterAction: "toggle",
	}
	popup.onEnter = func(p *Popup, chosen string) tea.Cmd {
		translator := runtimetooladapter.New(configs.SCRIPT_TOOL_PREFIX)
		translator.Scan(filesystem.ScriptToolsDir)
		if slices.Contains(translator.AlwaysAllowNames(), chosen) {
			p.subtitle = warnStyle.Render(chosen + " sets always_allow in its tool.json; edit that file to change it")
			return nil
		}
		if _, err := agentsexecallowtool.Toggle(chosen, ""); err != nil {
			p.subtitle = errorStyle.Render(fmt.Sprintf("allow %s: %v", chosen, err))
			return nil
		}
		fillAllowTools(p)
		return nil
	}
	fillAllowTools(popup)
	return popup
}

func fillAllowTools(p *Popup) {
	translator := runtimetooladapter.New(configs.SCRIPT_TOOL_PREFIX)
	translator.Scan(filesystem.ScriptToolsDir)
	fixed := translator.AlwaysAllowNames()
	allowed := agentsexecallowtool.Read()

	var names []string
	for _, tool := range translator.GetTools() {
		if fn, ok := tool["function"].(map[string]any); ok {
			if name, ok := fn["name"].(string); ok {
				names = append(names, name)
			}
		}
	}

	settings := make([]string, len(names))
	for i, name := range names {
		switch {
		case slices.Contains(fixed, name):
			settings[i] = okayStyle.Render("allow") + hintStyle.Render("  tool.json")
		case allowed[name]:
			settings[i] = okayStyle.Render("allow")
		default:
			settings[i] = hintStyle.Render("ask")
		}
	}
	p.subtitle = "allowed script tools run without asking  " + filesystem.AllowToolPath
	p.styledLines = nil
	if len(names) == 0 {
		p.styledLines = []string{hintStyle.Render("  no script tool installed yet")}
	}
	p.options = optionColumn(names, settings)
	p.values = names
	p.cursor = min(p.cursor, max(len(p.options)-1, 0))
}

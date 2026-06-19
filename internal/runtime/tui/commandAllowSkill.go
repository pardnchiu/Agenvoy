package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	allowSkill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

type AllowSkillScopeSelect struct {
	scope string
}

type AllowSkillPick struct {
	scope string
	name  string
}

func (t TUI) commandAllowSkill(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) > 1 {
		switch parts[1] {
		case "global":
			next, cmd := t.openAllowSkillPickerPopup("global")
			return next, cmd, true
		case "project":
			next, cmd := t.openAllowSkillPickerPopup("project")
			return next, cmd, true
		}
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Allow skill · scope",
		options: []string{"global   " + hintStyle.Render("~/.config/agenvoy/allow_skill"), "project  " + hintStyle.Render(".agenvoy/allow_skill")},
		values:  []string{"global", "project"},
		onConfirm: func(chosen string) any {
			return AllowSkillScopeSelect{scope: chosen}
		},
	}
	return t, nil, true
}

func (t TUI) openAllowSkillPickerPopup(scope string) (TUI, tea.Cmd) {
	scanner := agents.Scanner()
	if scanner == nil {
		return t, tea.Println(errorStyle.Render("[!] skill scanner unavailable") + "\n")
	}

	names := scanner.List()
	if len(names) == 0 {
		return t, tea.Println(hintStyle.Render("⎯ no skills available") + "\n")
	}

	var current map[string]bool
	switch scope {
	case "global":
		current = allowSkill.LoadGlobal()
	case "project":
		current = allowSkill.LoadEffective(t.cwd)
	default:
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] unknown scope: %s", scope)) + "\n")
	}

	sort.Strings(names)
	options := make([]string, len(names))
	values := make([]string, len(names))
	for i, name := range names {
		mark := "  "
		if current[name] {
			mark = hintStyle.Render("✓ ")
		}
		options[i] = mark + name
		values[i] = name
	}

	title := "Allow skill · " + scope
	if scope == "project" {
		title += "  " + hintStyle.Render("(✓ includes global)")
	}

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   title,
		options: options,
		values:  values,
		onConfirm: func(chosen string) any {
			return AllowSkillPick{scope: scope, name: chosen}
		},
	}
	return t, nil
}

func (t TUI) runAllowSkillToggle(scope, name string) (TUI, tea.Cmd) {
	var added bool
	var err error
	var pathLabel string
	switch scope {
	case "global":
		added, err = allowSkill.ToggleGlobal(name)
		pathLabel = filesystem.AllowSkillGlobalPath
	case "project":
		added, err = allowSkill.ToggleProject(t.cwd, name)
		pathLabel = filesystem.AllowSkillProjectPath(t.cwd)
	default:
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] unknown scope: %s", scope)) + "\n")
	}
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] allow-skill: %v", err)) + "\n")
	}
	verb := "removed"
	if added {
		verb = "added"
	}
	return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ allow_skill %s: %s (%s) · %s", verb, name, scope, pathLabel)) + "\n")
}

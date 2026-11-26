package tui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentsexecallowskill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

var remoteSkills = []string{
	"code-reviewer",
	"commit-generate",
	"go-test-generate",
	"readme-generate",
	"seo-optimize",
	"version-generate",
	"wiki-generate",
}

type SkillsInstallPick struct {
	names []string
}

type SkillSourcePick struct {
	enabled []string
}

type SkillsInstallDone struct {
	installed []string
	removed   []string
	failed    []string
}

func (t TUI) commandSkills() (TUI, tea.Cmd, bool) {
	next, cmd := t.openLoading("/skill", skillPopup)
	return next, cmd, true
}

func skillPopup() *Popup {
	installMulti := make(map[int]bool, len(remoteSkills))
	for i, name := range remoteSkills {
		if go_pkg_filesystem_reader.Exists(filepath.Join(filesystem.SystemDesignDir, name)) {
			installMulti[i] = true
		}
	}

	sourceMulti := make(map[int]bool, len(runtime.SkillSources))
	var disabled []string
	if cfg, err := config.Load(); err == nil {
		disabled = cfg.SkillSourceOff
	}
	for i, name := range runtime.SkillSources {
		sourceMulti[i] = !slices.Contains(disabled, name)
	}

	popup := &Popup{
		title:     "/skill",
		tabs:      []string{"permission", "system", "source"},
		openLabel: "folder",
		onOpen: func(chosen string) string {
			scanner := agents.Scanner()
			if scanner == nil {
				return ""
			}
			one := scanner.Lookup(chosen)
			if one == nil {
				return ""
			}
			return filepath.Dir(one.AbsPath)
		},
		onEnter: func(p *Popup, chosen string) tea.Cmd {
			if _, err := agentsexecallowskill.Toggle(chosen); err != nil {
				p.subtitle = errorStyle.Render(fmt.Sprintf("trust %s: %v", chosen, err))
				return nil
			}
			fillAllowSkills(p)
			return nil
		},
	}
	popup.onConfirm = func(chosen string) any {
		var names []string
		if chosen != "" {
			names = strings.Split(chosen, "\x1F")
		}
		if popup.tabIdx == 2 {
			return SkillSourcePick{enabled: names}
		}
		return SkillsInstallPick{names: names}
	}
	popup.onTab = func(p *Popup) tea.Cmd {
		p.cursor = 0
		p.styledLines = nil
		if p.tabIdx == 2 {
			p.kind = popupMultiSelect
			p.enterAction = ""
			p.subtitle = "checked: read  unchecked: skip  ~/.<source>/skills (claude also <cwd>/.claude/skills)"
			p.options = slices.Clone(runtime.SkillSources)
			p.values = runtime.SkillSources
			p.multi = sourceMulti
			return nil
		}
		if p.tabIdx == 1 {
			p.kind = popupMultiSelect
			p.enterAction = ""
			p.subtitle = "checked: git clone github.com/agenvoy/skill-<name>  unchecked: remove  " + filesystem.SystemDesignDir
			p.options = slices.Clone(remoteSkills)
			p.values = remoteSkills
			p.multi = installMulti
			return nil
		}
		p.kind = popupSingleSelect
		p.enterAction = "toggle"
		fillAllowSkills(p)
		return nil
	}
	popup.onTab(popup)
	return popup
}

func fillAllowSkills(p *Popup) {
	var names []string
	scanner := agents.Scanner()
	if scanner != nil {
		names = scanner.List()
		sort.Strings(names)
	}
	allowed := agentsexecallowskill.Load()
	labels := make([]string, len(names))
	settings := make([]string, len(names))
	for i, name := range names {
		source := ""
		if one := scanner.Lookup(name); one != nil {
			source = runtime.SkillSource(one.AbsPath)
		}
		labels[i] = name
		if source != "" && source != "system" {
			labels[i] = name + "  (" + source + ")"
		}
		settings[i] = hintStyle.Render("ask")
		if allowed[name] {
			settings[i] = okayStyle.Render("trust")
		}
	}
	p.subtitle = "trusted skills pass every read and write without asking; only sudo still asks  " + filesystem.TrustSkillPath
	p.styledLines = nil
	if len(names) == 0 {
		p.styledLines = []string{hintStyle.Render("  no skill installed yet")}
	}
	p.options = optionColumn(labels, settings)
	p.values = names
	p.cursor = min(p.cursor, max(len(p.options)-1, 0))
}

func (t TUI) runSkillsInstallPick(msg SkillsInstallPick) (TUI, tea.Cmd) {
	names := msg.names
	return t, func() tea.Msg {
		return syncSkills(names)
	}
}

func syncSkills(selected []string) SkillsInstallDone {
	var done SkillsInstallDone
	if err := go_pkg_filesystem.CheckDir(filesystem.SystemDesignDir, true); err != nil {
		done.failed = append(done.failed, fmt.Sprintf("%s: %v", filesystem.SystemDesignDir, err))
		return done
	}

	for _, name := range remoteSkills {
		dir := filepath.Join(filesystem.SystemDesignDir, name)
		exists := go_pkg_filesystem_reader.Exists(dir)
		want := slices.Contains(selected, name)

		switch {
		case want && !exists:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			out, err := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "https://github.com/agenvoy/skill-"+name+".git", dir).CombinedOutput()
			cancel()
			if err != nil {
				done.failed = append(done.failed, fmt.Sprintf("clone %s: %v %s", name, err, strings.TrimSpace(string(out))))
				continue
			}
			done.installed = append(done.installed, name)

		case !want && exists:
			if err := os.RemoveAll(dir); err != nil {
				done.failed = append(done.failed, fmt.Sprintf("remove %s: %v", name, err))
				continue
			}
			done.removed = append(done.removed, name)
		}
	}

	if scanner := agents.Scanner(); scanner != nil {
		scanner.Scan()
	}
	return done
}

func (t TUI) runSkillSourcePick(msg SkillSourcePick) (TUI, tea.Cmd) {
	dic, err := config.Get()
	if err != nil {
		return t, notice(msgError(fmt.Sprintf("skill source: %v", err)) + "\n")
	}
	disabled := []string{}
	for _, name := range runtime.SkillSources {
		if !slices.Contains(msg.enabled, name) {
			disabled = append(disabled, name)
		}
	}
	dic["skill_source_disabled"] = disabled
	if err := config.Write(dic); err != nil {
		return t, notice(msgError(fmt.Sprintf("skill source: %v", err)) + "\n")
	}
	if scanner := agents.Scanner(); scanner != nil {
		scanner.Scan()
	}
	return t, nil
}

func (t TUI) runSkillsInstallDone(msg SkillsInstallDone) (TUI, tea.Cmd) {
	slog.Debug("skills synced",
		slog.String("installed", strings.Join(msg.installed, ", ")),
		slog.String("removed", strings.Join(msg.removed, ", ")))
	var cmds []tea.Cmd
	for _, line := range msg.failed {
		cmds = append(cmds, notice(msgError(line)+"\n"))
	}
	if len(cmds) == 0 {
		return t, nil
	}
	return t, tea.Sequence(cmds...)
}

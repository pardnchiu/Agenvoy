package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	ToolName          = "run_skill"
	maxDescLen        = 512
	staticDescription = "Load a named skill's reference material into the current turn. Use when the system prompt's '## Skills' lists a skill that fits, or when the user names a skill. Result is advisory — integrate what fits."
)

type params struct {
	SkillName string `json:"skill"`
}

func registSelectSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name:        ToolName,
		Description: staticDescription,
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": map[string]any{
					"type":        "string",
					"description": "Exact skill name from the '## Skills' section of the system prompt.",
				},
			},
			"required": []string{"skill"},
		},
		Handler: handle,
	})
}

func ListBlock(scanner *runtime.SkillScanner, excludeSkills []string) string {
	if scanner == nil {
		return ""
	}
	names := scanner.List()
	if len(names) == 0 {
		return ""
	}

	excluded := make(map[string]bool, len(excludeSkills))
	for _, n := range excludeSkills {
		excluded[strings.TrimSpace(n)] = true
	}

	var b strings.Builder
	for _, n := range names {
		if excluded[n] {
			continue
		}
		desc := strings.TrimSpace(scanner.Skills.ByName[n].Description)
		if len([]rune(desc)) > maxDescLen {
			desc = string([]rune(desc)[:maxDescLen-1]) + "…"
		}
		b.WriteString("- ")
		b.WriteString(n)
		if desc != "" {
			b.WriteString(": ")
			b.WriteString(desc)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func handle(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
	if e.SkillScanner == nil {
		return "", fmt.Errorf("skill scanner unavailable in this execution context")
	}

	var p params
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
	}
	name := strings.TrimSpace(p.SkillName)

	if name == "" {
		return availableList(e.SkillScanner, "skill is required"), nil
	}

	s, ok := e.SkillScanner.Skills.ByName[name]
	if !ok {
		return availableList(e.SkillScanner, fmt.Sprintf("skill not found: %q", name)), nil
	}

	return RenderReference(s), nil
}

func RenderActivation(s *skill.Skill) string {
	content := resolveSkillPaths(s)

	var b strings.Builder
	fmt.Fprintf(&b, "active skill: %s\nskill directory: %s\n\n---\n\n", s.Name, s.Path)
	if ext := strings.TrimSpace(configs.SkillExecution); ext != "" {
		b.WriteString(ext)
		b.WriteString("\n\n---\n\n")
	}
	b.WriteString(content)
	return b.String()
}

// RenderReference returns the skill body without the skill_execution.md

func RenderReference(s *skill.Skill) string {
	content := resolveSkillPaths(s)

	var b strings.Builder
	fmt.Fprintf(&b, "skill: %s\nskill directory: %s\n\n---\n\n", s.Name, s.Path)
	b.WriteString(content)
	return b.String()
}

func resolveSkillPaths(s *skill.Skill) string {
	content := s.Content
	for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
		resolved := filepath.Join(s.Path, prefix)
		if go_pkg_filesystem_reader.Exists(resolved) {
			content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
		}
	}
	return content
}

func availableList(scanner *runtime.SkillScanner, reason string) string {
	names := scanner.List()
	if len(names) == 0 {
		return reason + "; no skills available on this host"
	}

	var b strings.Builder
	b.WriteString(reason)
	b.WriteString("\navailable skills:\n")
	for _, n := range names {
		desc := strings.TrimSpace(scanner.Skills.ByName[n].Description)
		if len([]rune(desc)) > maxDescLen {
			desc = string([]rune(desc)[:maxDescLen-1]) + "…"
		}
		b.WriteString("- ")
		b.WriteString(n)
		if desc != "" {
			b.WriteString(": ")
			b.WriteString(desc)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

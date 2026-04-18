package skillTool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/skill"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const ToolName = "select_skill"

const maxDescLen = 200

const staticDescription = "Activate a skill by exact name. Available skill names are listed in the system prompt under '## Skills'. The tool result returns the skill body plus execution guidance — treat it as binding instructions for subsequent iterations. Pass skill='none' to clear. When the user's request matches a listed skill (exact name or obvious alias), call this tool immediately instead of answering from prior knowledge."

type params struct {
	SkillName string `json:"skill"`
}

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        ToolName,
		Description: staticDescription,
		ReadOnly:    true,
		AlwaysLoad:  true,
		Concurrent:  false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": map[string]any{
					"type":        "string",
					"description": "Exact skill name from the '## Skills' section of the system prompt, or 'none' to clear.",
				},
			},
			"required": []string{"skill"},
		},
		Handler: handle,
	})
}

func ListBlock(scanner *skill.SkillScanner) string {
	if scanner == nil {
		return ""
	}
	names := scanner.List()
	if len(names) == 0 {
		return ""
	}

	var b strings.Builder
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

	if strings.EqualFold(name, "none") {
		e.ActiveSkill = nil
		return "active skill cleared", nil
	}

	s, ok := e.SkillScanner.Skills.ByName[name]
	if !ok {
		return availableList(e.SkillScanner, fmt.Sprintf("skill not found: %q", name)), nil
	}

	e.ActiveSkill = s
	return renderSkill(s), nil
}

func renderSkill(s *skill.Skill) string {
	content := s.Content
	for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
		resolved := filepath.Join(s.Path, prefix)
		if _, err := os.Stat(resolved); err == nil {
			content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "active skill: %s\nskill directory: %s\n\n---\n\n", s.Name, s.Path)
	if ext := strings.TrimSpace(configs.SkillExecution); ext != "" {
		b.WriteString(ext)
		b.WriteString("\n\n---\n\n")
	}
	b.WriteString(content)
	return b.String()
}

func availableList(scanner *skill.SkillScanner, reason string) string {
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

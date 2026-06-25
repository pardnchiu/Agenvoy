package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRunSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name: "run_skill",
		Description: `
Load a named skill's reference material into the current turn.
Use when the system prompt's '## Skills' lists a skill that fits, or when the user names a skill.
Result is advisory — integrate what fits.`,
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
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			if e.SkillScanner == nil {
				return "", fmt.Errorf("skill scanner unavailable in this execution")
			}

			if len(args) < 1 {
				return "", fmt.Errorf("arguments are required")
			}

			var params struct {
				Skill string `json:"skill"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Skill)
			if name == "" {
				return "", fmt.Errorf("skill is required")
			}
			skill, ok := e.SkillScanner.Skills.ByName[name]
			if !ok {
				return availableList(e.SkillScanner), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "skill: %s\nskill directory: %s\n\n---\n\n", skill.Name, skill.Path)
			sb.WriteString(skill.Resolved())
			return sb.String(), nil
		},
	})
}

func availableList(scanner *runtime.SkillScanner) string {
	skills := scanner.List()
	if len(skills) == 0 {
		return "no skills available on this host"
	}

	var sb strings.Builder
	sb.WriteString("skill not found\navailable skills:\n")
	for _, skill := range skills {
		sb.WriteString("- ")
		sb.WriteString(skill)
		if desc := go_pkg_utils.TruncateString(scanner.Skills.ByName[skill].Description, 512); desc != "" {
			sb.WriteString(": ")
			sb.WriteString(desc)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

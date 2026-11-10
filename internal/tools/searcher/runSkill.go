package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRunSkill() {
	toolregister.Regist(toolregister.Def{
		Name:        "run_skill",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `Loads a named skill's reference material and its directory path into this turn; an unknown name returns the available skills instead. What comes back is reference, not a script to execute line by line: take the parts that fit.
Use when a skill in the available-skills list fits the task, or the user names one ("use the X skill", "/X").
Changing a skill → edit_skill.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": map[string]any{
					"type":        "string",
					"description": "Exact skill name from the available-skills list.",
				},
			},
			"required": []string{"skill"},
		},
		Handler: func(_ context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			if e.SkillScanner == nil {
				return "", utilsmsg.New(utilsmsg.Unavailable, "skill scanner", "unavailable in this execution")
			}

			if len(args) < 1 {
				return "", utilsmsg.New(utilsmsg.Invalid, "arguments", "required")
			}

			var params struct {
				Skill string `json:"skill"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			name := strings.TrimSpace(params.Skill)
			if name == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "skill", "required")
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

package git

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func skillCommit() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "skill_git_commit",
		Description: "Commit all changes under ~/.config/agenvoy/skills to git using the required commit message format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"act": map[string]any{
					"type":        "string",
					"enum":        []string{"add", "update"},
					"description": "Action type: 'add' for a new skill, 'update' for an existing skill.",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Skill name in hyphen-case (for example, 'my-skill').",
				},
			},
			"required": []string{"act", "skill_name"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Act       string `json:"act"`
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Act != "add" && params.Act != "update" {
				return "", fmt.Errorf("act must be 'add' or 'update', got: %s", params.Act)
			}
			if params.SkillName == "" {
				return "", fmt.Errorf("skill_name is required")
			}
			if err := filesystem.CheckSkillsGit(ctx); err != nil {
				return "", fmt.Errorf("filesystem.CheckSkillsGit: %w", err)
			}
			if err := filesystem.CommitSkills(ctx, params.Act, params.SkillName); err != nil {
				return "", fmt.Errorf("filesystem.CommitSkills: %w", err)
			}
			return fmt.Sprintf("committed: %s_%s", params.Act, params.SkillName), nil
		},
	})
}

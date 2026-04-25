package git

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func skillRollback() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "skill_git_rollback",
		Description: "Roll back ~/.config/agenvoy/skills to the specified git commit.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"commit": map[string]any{
					"type":        "string",
					"description": "Target commit hash (at least 7 characters) or ref such as HEAD~1.",
				},
			},
			"required": []string{"commit"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Commit string `json:"commit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Commit == "" {
				return "", fmt.Errorf("commit is required")
			}
			out, err := filesystem.RollbackSkills(ctx, params.Commit)
			if err != nil {
				return "", fmt.Errorf("filesystem.RollbackSkills: %w", err)
			}
			return out, nil
		},
	})
}

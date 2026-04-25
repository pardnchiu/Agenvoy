package git

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func skillLog() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "skill_git_log",
		ReadOnly:    true,
		Description: "List the git commit history for ~/.config/agenvoy/skills in oneline format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of commits to show. Default 20, maximum 100.",
					"default":     20,
				},
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Limit int `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Limit > 100 {
				params.Limit = 100
			}
			log, err := filesystem.LogSkills(ctx, params.Limit)
			if err != nil {
				return "", fmt.Errorf("filesystem.LogSkills: %w", err)
			}
			if log == "" {
				return "no commits yet", nil
			}
			return log, nil
		},
	})
}

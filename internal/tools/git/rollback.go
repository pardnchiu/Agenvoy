package git

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRollback() {
	toolRegister.Regist(toolRegister.Def{
		Name: "git_rollback",
		Description: `
Reset skills or tools directory to a prior git commit (hard reset).
Use to revert a broken auto-commit or unwanted self-improvement.
Run git_log first to identify the target commit hash.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tag": map[string]any{
					"type":        "string",
					"enum":        []string{"skills", "tools"},
					"description": "Target repo. 'skills' = ~/.config/agenvoy/skills, 'tools' = ~/.config/agenvoy/tools.",
				},
				"commit": map[string]any{
					"type":        "string",
					"description": "Commit hash (≥7 chars) or ref to reset to. Get from git_log (e.g. 'a1b2c3d', 'HEAD~1').",
				},
			},
			"required": []string{"tag", "commit"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Tag    string `json:"tag"`
				Commit string `json:"commit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			target, err := matchTarget(params.Tag)
			if err != nil {
				return "", err
			}

			if params.Commit == "" {
				return "", fmt.Errorf("commit is required")
			}

			result, err := filesystem.GitRollback(ctx, target, params.Commit)
			if err != nil {
				return "", fmt.Errorf("internal/filesystem: GitRollback [%s]: %w", params.Tag, err)
			}
			return result, nil
		},
	})
}

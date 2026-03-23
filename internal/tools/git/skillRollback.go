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
		Description: "將 ~/.config/agenvoy/skills 回朔至指定的 git commit。使用 skill_git_log 取得 commit hash 後呼叫。此操作不可逆，會丟失該 commit 之後的所有變更。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"commit": map[string]any{
					"type":        "string",
					"description": "目標 commit hash（至少 7 字元）或 ref（例如 HEAD~1）",
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

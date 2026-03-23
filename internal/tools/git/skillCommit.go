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
		Description: "將 ~/.config/agenvoy/skills 目錄下的所有變更提交至 git。commit message 格式為 {act}_{skill_name}_{YYYYMMDD}。act 只能是 'add' 或 'update'。每次新增或修改 skill 後必須呼叫。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"act": map[string]any{
					"type":        "string",
					"enum":        []string{"add", "update"},
					"description": "操作類型：'add' 代表新增 skill，'update' 代表修改既有 skill",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Skill 名稱，使用 hyphen-case（例如 'my-skill'）",
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

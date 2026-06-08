package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_skill",
		AlwaysAllow: true,
		Description: `
Replace an exact string match inside a skill file under ~/.config/agenvoy/skills/.
Use for targeted edits; write_skill for full rewrite.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Relative path under skills dir (e.g. 'my-skill/SKILL.md', 'my-skill/scripts/01.md').",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact string to replace, including indentation. Must be unique unless replace_all is true.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement string. Empty string deletes old_string.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace all occurrences. Defaults to false.",
					"default":     false,
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			path := strings.TrimSpace(params.Path)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			if params.OldString == "" {
				return "", fmt.Errorf("old_string is required")
			}
			if params.OldString == params.NewString {
				return "", fmt.Errorf("no edit needed")
			}

			absPath := filepath.Clean(filepath.Join(filesystem.SkillsDir, path))
			if !strings.HasPrefix(absPath, filesystem.SkillsDir+string(filepath.Separator)) {
				return "", fmt.Errorf("path must stay within skills dir")
			}

			if err := patch(absPath, params.OldString, params.NewString, params.ReplaceAll); err != nil {
				return "", err
			}

			filesystem.GitAutoCommitByPath(ctx, filesystem.GitSkills, absPath, false)
			return fmt.Sprintf("updated: %s", absPath), nil
		},
	})
}

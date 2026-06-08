package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_skill",
		Description: `
Move a skill directory to ~/.config/agenvoy/skills/.Trash/.
Use when a skill is obsolete or must be rebuilt; recoverable via git_rollback.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Skill directory name, single segment (e.g. 'my-skill').",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}

			dir := filepath.Join(filesystem.SkillsDir, name)
			if !go_pkg_filesystem_reader.IsDir(dir) {
				return "", fmt.Errorf("skill %q does not exist", name)
			}

			dst, err := filesystem.TrashDir(dir, filesystem.SkillTrashDir, name)
			if err != nil {
				return "", err
			}

			filesystem.GitAutoCommit(ctx, filesystem.GitSkills, "trash", name)
			return fmt.Sprintf("trashed: %s → %s", dir, dst), nil
		},
	})
}

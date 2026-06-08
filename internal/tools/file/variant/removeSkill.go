package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_skill",
		Description: `
Remove a skill directory under ~/.config/agenvoy/skills/<name>/.
Use when a skill is no longer needed or must be rebuilt from scratch.
Auto-commits to skill git after removal.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Skill directory name (e.g. 'my-skill').",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			if strings.Contains(name, "/") || strings.Contains(name, "..") {
				return "", fmt.Errorf("name must be a single directory name")
			}

			dir := filepath.Join(filesystem.SkillsDir, name)
			if !go_pkg_filesystem_reader.IsDir(dir) {
				return "", fmt.Errorf("skill %q does not exist", name)
			}

			if err := os.RemoveAll(dir); err != nil {
				return "", fmt.Errorf("os.RemoveAll [%s]: %w", dir, err)
			}

			skill.AutoCommit(ctx, "remove", name)
			return fmt.Sprintf("removed: %s", dir), nil
		},
	})
}

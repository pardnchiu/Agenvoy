package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_skill",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `
Create or fully rewrite a file under ~/.config/agenvoy/skills/.
Use for new skill files or full rewrites; patch_skill for targeted edits.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Relative path under skills dir (e.g. 'my-skill/SKILL.md', 'my-skill/scripts/01.md').",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full file content to write. Must be complete, not a diff.",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			path := strings.TrimSpace(params.Path)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			if strings.TrimSpace(params.Content) == "" {
				return "", fmt.Errorf("content is required")
			}

			absPath := filepath.Clean(filepath.Join(filesystem.SkillsDir, path))
			if !strings.HasPrefix(absPath, filesystem.SkillsDir+string(filepath.Separator)) {
				return "", fmt.Errorf("path must stay within skills dir")
			}

			if err := go_pkg_filesystem.WriteFile(absPath, params.Content, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/agenvoy/internal/filesystem: WriteFile [%s]: %w", absPath, err)
			}

			filesystem.GitAutoCommitByPath(ctx, filesystem.GitSkills, absPath, true)
			return fmt.Sprintf("created: %s", absPath), nil
		},
	})
}

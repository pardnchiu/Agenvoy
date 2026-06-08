package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchSkill() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_skill",
		AlwaysAllow: true,
		Description: `
Replace an exact string match inside a file under ~/.config/agenvoy/skills/.
Use when editing an existing skill file (SKILL.md, scripts/, templates/, etc.).
Auto-commits to skill git after each patch.
For full rewrite use write_skill.`,
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
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			path := strings.TrimSpace(params.Path)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}

			old := params.OldString
			new := params.NewString
			if old == "" {
				return "", fmt.Errorf("old_string is required")
			}
			if old == new {
				return "", fmt.Errorf("no edit needed")
			}

			absPath := filepath.Join(filesystem.SkillsDir, path)
			absPath, err := filepath.Abs(absPath)
			if err != nil {
				return "", fmt.Errorf("filepath.Abs: %w", err)
			}
			if !strings.HasPrefix(absPath, filesystem.SkillsDir+string(filepath.Separator)) {
				return "", fmt.Errorf("path must stay within skills dir")
			}

			info, err := os.Stat(absPath)
			if err != nil {
				return "", fmt.Errorf("os.Stat: %w", err)
			}
			if info.Size() > maxReadSize {
				return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
			}

			content, err := go_pkg_filesystem.ReadText(absPath)
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem ReadText [%s]: %w", absPath, err)
			}

			if !strings.Contains(content, old) {
				return "", fmt.Errorf("%s is not found in %s", old, absPath)
			}

			search := old
			if new == "" && !strings.HasSuffix(old, "\n") && strings.Contains(content, old+"\n") {
				search = old + "\n"
			}
			var updated string
			if params.ReplaceAll {
				updated = strings.ReplaceAll(content, search, new)
			} else {
				updated = strings.Replace(content, search, new, 1)
			}

			if err := go_pkg_filesystem.WriteFile(absPath, updated, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile [%s]: %w", absPath, err)
			}

			skill.AutoCommitByPath(ctx, absPath, false)
			return fmt.Sprintf("updated: %s", absPath), nil
		},
	})
}

package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchEdit() {
	toolRegister.Regist(toolRegister.Def{
		Name: "patch_edit",
		Description: `
Replace an exact string match inside a file.
Apply targeted edits to an existing file.
Accepts absolute paths and '~' (e.g. '/abs/path/foo.go', '~/notes.md').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to edit (e.g. '/abs/path/foo.go', '~/notes.md', 'relative/file.md').",
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
					"description": "If true, replace all occurrences (e.g. when renaming a variable). Defaults to false.",
					"default":     false,
				},
			},
			"required": []string{
				"path",
				"old_string",
				"new_string",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			absPath, err := filesystem.AbsPath(baseDir, params.Path, false)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path or name is required")
			}

			// * not to trim string, avoid user use " " to indicate indent
			old := params.OldString
			new := params.NewString
			if old == "" {
				return "", fmt.Errorf("old_string is required")
			}

			if old == new {
				return "", fmt.Errorf("no edit needed")
			}
			return patchEditHandler(ctx, absPath, old, new, params.ReplaceAll)
		},
	})
}

func patchEditHandler(ctx context.Context, path, old, new string, replaceAll bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	fileContent := string(fileBytes)
	matchCount := strings.Count(fileContent, old)
	if matchCount == 0 {
		return "", fmt.Errorf("%s is not found in %s", old, path)
	}

	newContent := old
	if new == "" && !strings.HasSuffix(old, "\n") && strings.Contains(fileContent, old+"\n") {
		newContent = old + "\n"
	}
	if replaceAll {
		newContent = strings.ReplaceAll(fileContent, newContent, new)
	} else {
		newContent = strings.Replace(fileContent, newContent, new, 1)
	}

	if err := filesystem.WriteFile(path, newContent, 0644); err != nil {
		return "", fmt.Errorf("filesystem.WriteFile: %w", err)
	}

	if filesystem.IsSkillsDir(path) {
		skillName := filesystem.GetSkillName(path)
		if err := filesystem.CheckSkillsGit(ctx); err == nil {
			_ = filesystem.CommitSkills(ctx, "update", skillName)
		}
	}
	return fmt.Sprintf("successfully updated %s", path), nil
}

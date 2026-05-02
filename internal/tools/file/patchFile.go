package file

import (
	"context"
	"encoding/json"
	"fmt"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	filewriter "github.com/pardnchiu/agenvoy/internal/filesystem/fileWriter"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchFile() {
	toolRegister.Regist(toolRegister.Def{
		Name: "patch_file",
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

			absPath, err := go_pkg_filesystem.AbsPath(baseDir, params.Path, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
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
			return filewriter.Patch(ctx, absPath, old, new, params.ReplaceAll)
		},
	})
}

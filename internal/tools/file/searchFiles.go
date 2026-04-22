package file

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/fileReader"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registSearchFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "search_files",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Search file contents by RE2 regex within a directory.
Locate code or text when the matching string is known but the file is not.
Scope with file_pattern glob (e.g. '**/*.go', 'configs/**').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir": map[string]any{
					"type":        "string",
					"description": "Directory to search in (e.g. '.', '~/downloads', '/abs/path'). Defaults to current working directory.",
				},
				"pattern": map[string]any{
					"type":        "string",
					"description": "RE2 regex matched per line (e.g. 'func\\s+\\w+Handler', 'TODO:', 'api_key').",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "Glob relative to dir to narrow files (e.g. '**/*.go', 'configs/**/*.json').",
				},
			},
			"required": []string{
				"pattern",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Dir         string `json:"dir"`
				Pattern     string `json:"pattern"`
				FilePattern string `json:"file_pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			dir := strings.TrimSpace(params.Dir)
			if dir == "" {
				dir = e.WorkDir
			}

			absPath, err := filesystem.AbsPath(dir, "", false)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}

			textPattern := strings.TrimSpace(params.Pattern)
			if textPattern == "" {
				return "", fmt.Errorf("pattern is required")
			}

			var filePatterns []string
			if params.FilePattern != "" {
				filePatterns = strings.Split(filepath.ToSlash(params.FilePattern), "/")
			}
			return fileReader.SearchFiles(absPath, textPattern, filePatterns)
		},
	})
}

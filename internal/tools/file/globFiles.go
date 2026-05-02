package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func registGlobFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "glob_files",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Find files matching a glob pattern within a directory.
Locate specific file types (e.g. '**/*.go' for Go files).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir": map[string]any{
					"type":        "string",
					"description": "Directory to search in (e.g. '.', '~/downloads', '/abs/path'). Defaults to current working directory.",
					"default":     ".",
				},
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern relative to dir (e.g. '**/*.go', '*.md'). No leading '/' or '~' — put absolute paths in dir.",
				},
			},
			"required": []string{
				"pattern",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Dir     string `json:"dir"`
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			dir := strings.TrimSpace(params.Dir)
			absPath, err := go_pkg_filesystem.AbsPath(e.WorkDir, dir, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
			}

			pattern := strings.TrimSpace(params.Pattern)
			if pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}

			matches, err := go_pkg_filesystem_reader.GlobFiles(absPath, pattern)
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem_reader.GlobFiles: %w", err)
			}
			data, err := json.Marshal(matches)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(data), nil
		},
	})
}

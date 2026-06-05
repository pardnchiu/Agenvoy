package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/tools/file/denied"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func registGlobFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "glob_files",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Find files matching a glob pattern within a directory (e.g. '**/*.go'). Use when only a filename or partial path is known — never guess full paths. Call read_file on each match before editing to confirm the correct file.",
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
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
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

			if parent, ok := denied.Hit(e.SessionID, absPath); ok {
				return "", fmt.Errorf("permission denied: %s is under previously rejected %s; not retried", absPath, parent)
			}

			pattern := strings.TrimSpace(params.Pattern)
			if pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}

			matches, err := go_pkg_filesystem_reader.GlobFiles(absPath, pattern)
			if err != nil {
				if denied.IsPermission(err) {
					denied.Register(e.SessionID, absPath)
					return "", fmt.Errorf("permission denied: %s (recorded; further reads under this path will be skipped)", absPath)
				}
				return "", fmt.Errorf("go_pkg_filesystem_reader.GlobFiles: %w", err)
			}
			if err := ctx.Err(); err != nil {
				return "", err
			}
			raw, err := json.Marshal(matches)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

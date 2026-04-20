package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type file struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func registGlobFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "glob_files",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Find files matching a glob pattern.
Locate specific file types (e.g. '**/*.go' for Go files).
Accepts absolute paths and '~' (e.g. '~/tsmc_report*').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern (e.g. '**/*.go', '~/tsmc_report*', '/abs/path/**/*.md').",
				},
			},
			"required": []string{
				"pattern",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			pattern := strings.TrimSpace(params.Pattern)
			if pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}
			return handlerGlobFiles(e, pattern)
		},
	})
}

func handlerGlobFiles(e *toolTypes.Executor, pattern string) (string, error) {
	baseStr, globParts := splitGlob(pattern)
	baseDir := e.WorkDir
	if baseStr != "" {
		abs, err := filesystem.AbsPath(e.WorkDir, baseStr, false)
		if err != nil {
			return "", fmt.Errorf("filesystem.AbsPath: %w", err)
		}
		baseDir = abs
	}

	results := []file{}

	var matches []string
	if slices.Contains(globParts, "**") {
		walked, err := filesystem.WalkFiles(e.WorkDir, baseDir)
		if err != nil {
			return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
		}

		for _, rel := range walked {
			parts := strings.Split(rel, "/")
			if !filesystem.IsMatch(globParts, parts) {
				continue
			}
			matches = append(matches, filepath.Join(baseDir, rel))
		}
	} else {
		// * fast path: no `**`, delegate to filepath.Glob
		absPattern := filepath.Join(append([]string{baseDir}, globParts...)...)
		files, err := filepath.Glob(absPattern)
		if err != nil {
			return "", fmt.Errorf("filepath.Glob: %w", err)
		}
		matches = files
	}

	for _, full := range matches {
		info, err := os.Stat(full)
		if err != nil {
			continue
		}

		results = append(results, file{
			Name:    info.Name(),
			Path:    full,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}

	data, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(data), nil
}

func splitGlob(pattern string) (string, []string) {
	parts := strings.Split(pattern, "/")
	var baseParts, globParts []string
	foundMeta := false
	for _, p := range parts {
		if !foundMeta && !strings.ContainsAny(p, "*?[") {
			baseParts = append(baseParts, p)
			continue
		}
		foundMeta = true
		globParts = append(globParts, p)
	}

	baseStr := strings.Join(baseParts, "/")
	if strings.HasPrefix(pattern, "/") && !strings.HasPrefix(baseStr, "/") {
		baseStr = "/" + baseStr
	}
	return baseStr, globParts
}

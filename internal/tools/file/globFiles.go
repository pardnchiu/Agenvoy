package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registGlobFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "glob_files",
		ReadOnly:    true,
		Description: "Find files matching a glob pattern. Use to locate specific file types (e.g. '**/*.go' for all Go files). Supports absolute paths and `~` for user home (e.g. '~/tsmc_report*').",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files against (e.g. '**/*.go', 'src/**/*.ts', '*.md', '~/tsmc_report*', '/Users/me/notes/**/*.txt')",
				},
			},
			"required": []string{"pattern"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Pattern == "" {
				return "", fmt.Errorf("pattern is required")
			}

			pattern := filepath.ToSlash(params.Pattern)
			baseStr, globParts := splitGlob(pattern)

			baseDir := e.WorkDir
			if baseStr != "" {
				abs, err := filesystem.AbsPath(e.WorkDir, baseStr, false)
				if err != nil {
					return "", fmt.Errorf("filesystem.AbsPath: %w", err)
				}
				baseDir = abs
			}

			// * no glob metacharacters — direct stat
			if len(globParts) == 0 {
				info, err := os.Stat(baseDir)
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Sprintf("%s no files found", pattern), nil
					}
					return "", fmt.Errorf("os.Stat: %w", err)
				}
				if info.IsDir() {
					return fmt.Sprintf("%s no files found (path is a directory)", pattern), nil
				}
				return fmt.Sprintf("%s / %s\n", baseDir, info.ModTime().Format("2006-01-02 15:04")), nil
			}

			var matches []string
			if hasDoubleStar(globParts) {
				files, err := filesystem.WalkFiles(e.WorkDir, baseDir)
				if err != nil {
					return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
				}
				for _, file := range files {
					parts := strings.Split(file, "/")
					if !filesystem.IsMatch(globParts, parts) {
						continue
					}
					matches = append(matches, filepath.Join(baseDir, file))
				}
			} else {
				// * fast path: no `**`, delegate to filepath.Glob
				absPattern := filepath.Join(append([]string{baseDir}, globParts...)...)
				raw, err := filepath.Glob(absPattern)
				if err != nil {
					return "", fmt.Errorf("filepath.Glob: %w", err)
				}
				matches = raw
			}

			var sb strings.Builder
			for _, full := range matches {
				info, err := os.Stat(full)
				if err != nil || info.IsDir() {
					continue
				}
				sb.WriteString(full)
				sb.WriteString(" / ")
				sb.WriteString(info.ModTime().Format("2006-01-02 15:04"))
				sb.WriteByte('\n')
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("%s no files found", pattern), nil
			}
			return sb.String(), nil
		},
	})
}

func splitGlob(pattern string) (string, []string) {
	parts := strings.Split(pattern, "/")
	var baseParts, globParts []string
	foundMeta := false
	for _, p := range parts {
		if !foundMeta && !containsMeta(p) {
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

func containsMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func hasDoubleStar(parts []string) bool {
	for _, p := range parts {
		if p == "**" {
			return true
		}
	}
	return false
}

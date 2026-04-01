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
		Description: "Find files matching a glob pattern. Use to locate specific file types (e.g. '**/*.go' for all Go files).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match files against (e.g. '**/*.go', 'src/**/*.ts', '*.md')",
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

			pattern := filepath.ToSlash(params.Pattern)
			patterns := strings.Split(pattern, "/")
			files, err := filesystem.WalkFiles(e.WorkDir)
			if err != nil {
				return "", err
			}

			var sb strings.Builder
			for _, file := range files {
				parts := strings.Split(file, "/")
				if !filesystem.IsMatch(patterns, parts) {
					continue
				}
				sb.WriteString(file)
				if info, err := os.Stat(filepath.Join(e.WorkDir, file)); err == nil {
					sb.WriteString(" / ")
					sb.WriteString(info.ModTime().Format("2006-01-02 15:04"))
				}
				sb.WriteByte('\n')
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("%s no files found", pattern), nil
			}
			return sb.String(), nil
		},
	})
}

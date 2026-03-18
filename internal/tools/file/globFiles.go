package file

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registGlobFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "glob_files",
		Description: "尋找符合 glob 模式的檔案。用於尋找特定檔案類型（例如 '**/*.go' 表示所有 Go 檔案）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "用於比對檔案的 Glob 模式（例如 '**/*.go'、'src/**/*.ts'、'*.md'）",
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
				if filesystem.IsMatch(patterns, parts) {
					sb.WriteString(file + "\n")
				}
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("%s no files found", pattern), nil
			}
			return sb.String(), nil
		},
	})
}

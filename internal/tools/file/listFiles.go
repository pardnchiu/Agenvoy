package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_files",
		Description: "列出指定路徑的檔案和目錄。用於探索專案結構。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要列出的目錄路徑（相對於專案根目錄或絕對路徑）。使用 '.' 表示目前目錄。",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "如果為 true，則遞迴列出檔案。預設為 false。",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path      string `json:"path"`
				Recursive bool   `json:"recursive"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			var files []string
			var err error
			if params.Recursive {
				files, err = filesystem.WalkFiles(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
				}
			} else {
				files, err = filesystem.ListDir(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.ListDir: %w", err)
				}
			}
			return fmt.Sprintf("[%s]", strings.Join(files, ",")), nil
		},
	})
}

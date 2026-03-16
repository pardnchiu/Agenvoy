package file

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_file",
		Description: "將內容寫入檔案。如果檔案不存在則建立，如果存在則覆寫。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要寫入的檔案路徑（相對於專案根目錄或絕對路徑）",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "要寫入檔案的內容",
				},
			},
			"required": []string{"path", "content"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}

			if err := filesystem.WriteFile(e.WorkPath, params.Path, params.Content, 0644); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}
			return fmt.Sprintf("Successfully wrote file: %s", params.Path), nil
		},
	})
}

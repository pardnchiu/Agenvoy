package scriptTools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdateScript() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "update_script",
		Description: "覆寫已存在的排程腳本內容。用於修改現有腳本，不改變檔名，不影響已設定的排程。先用 read_script 確認內容後再呼叫。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "要覆寫的腳本檔名（含副檔名，不含路徑），例如 'notify_1741569300.sh'",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "新的腳本內容",
				},
			},
			"required": []string{"name", "content"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}
			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}
			if err := filesystem.WriteFile(filepath.Join(filesystem.ScriptsDir, params.Name), params.Content, 0755); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}
			return fmt.Sprintf("script updated: %s", params.Name), nil
		},
	})
}

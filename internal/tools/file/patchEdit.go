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

func registPatchEdit() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_edit",
		Description: "透過精確字串匹配來編輯檔案。僅替換第一個匹配項。適合對檔案進行小幅修改，比 write_file 更安全。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要編輯的檔案路徑（相對於專案根目錄或絕對路徑）",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "要被替換的原始內容（必須精確匹配）",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "替換為的新內容",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path      string `json:"path"`
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			content, absPath, err := readFile(e, params.Path)
			if err != nil {
				return "", fmt.Errorf("readFile: %w", err)
			}

			if !strings.Contains(content, params.OldString) {
				return "", fmt.Errorf("%s is not found in %s", params.OldString, absPath)
			}

			newContent := strings.Replace(content, params.OldString, params.NewString, 1)
			if err := filesystem.WriteFile(absPath, newContent, 0644); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}
			return fmt.Sprintf("%s updated", absPath), nil
		},
	})
}

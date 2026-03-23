package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_file",
		Description: "將內容寫入檔案。如果檔案不存在則建立，如果存在則覆寫。寫入 ~/.config/agenvoy/skills 下的檔案時會自動 git commit。未指定目錄時，路徑須以 ~/.config/agenvoy/download/<檔名> 為基底。",
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
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
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

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}
			absPath, err := filesystem.GetAbsPath(baseDir, params.Path)
			if err != nil {
				return "", fmt.Errorf("filesystem.GetAbsPath: %w", err)
			}

			_, statErr := os.Stat(absPath)
			isNew := os.IsNotExist(statErr)

			if err := filesystem.WriteFile(absPath, params.Content, 0644); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}

			if filesystem.IsSkillsDir(absPath) {
				act := "update"
				if isNew {
					act = "add"
				}
				skillName := filesystem.GetSkillName(absPath)
				if err := filesystem.CheckSkillsGit(ctx); err == nil {
					_ = filesystem.CommitSkills(ctx, act, skillName)
				}
			}

			return fmt.Sprintf("%s wrote", absPath), nil
		},
	})
}

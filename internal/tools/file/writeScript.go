package file

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteScript() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_script",
		Description: "在 ~/.config/agenvoy/scheduler/scripts/ 建立腳本檔案（.sh 或 .py）。回傳值為實際儲存的檔名（含 UTC timestamp 後綴，例如 notify_1741569300.sh），必須將此回傳檔名傳給 add_task 或 add_cron 的 script 參數。腳本須以 #!/bin/sh 或 #!/usr/bin/env python3 開頭。**同一個回傳檔名可重複傳給多個 add_cron 呼叫**（例如建立三個不同時間點的 cron 只需寫一次腳本，將同一檔名分別傳給三次 add_cron）。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "腳本檔名，只填檔名不含路徑，必須以 .sh 或 .py 結尾，例如 'notify.sh'、'backup.py'",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "腳本內容，.sh 必須以 #!/bin/sh 開頭，.py 必須以 #!/usr/bin/env python3 開頭",
				},
			},
			"required": []string{"name", "content"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}

			ext := strings.ToLower(filepath.Ext(params.Name))
			if ext != ".sh" && ext != ".py" {
				return "", fmt.Errorf("scripts only support .sh or .py")
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}

			base := strings.TrimSuffix(params.Name, ext)
			uniqueName := fmt.Sprintf("%s_%d%s", base, time.Now().UTC().Unix(), ext)
			path := filepath.Join(filesystem.ScriptsDir, uniqueName)

			if err := filesystem.WriteFile(path, params.Content, 0755); err != nil {
				return "", fmt.Errorf("filesystem.WriteFile: %w", err)
			}

			return fmt.Sprintf(`script saved. pass "%s" as the script parameter to add_task or add_cron`, uniqueName), nil
		},
	})
}

package browser

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "fetch_page",
		Description: "使用 Chrome 瀏覽器開啟網頁，等待 JS 完整渲染後擷取主要內容，以純文字 Markdown 格式返回給 agent 閱讀。此工具不會寫入任何檔案，內容僅存在於 agent context 中。【適用情境】查詢、摘要、分析、爬取內容、回答問題、任何需要讀取網頁內容的任務。【禁止情境】使用者明確說「存檔」、「存到本地」、「儲存成檔案」、「下載到 xxx 路徑」時，改用 download_page。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "要擷取內容的完整網址（需包含 https://）",
				},
			},
			"required": []string{"url"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Load(params.URL)
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "download_page",
		Description: "【嚴格限制：僅在使用者明確要求將網頁儲存到本地端檔案時使用】使用 Chrome 瀏覽器取得完整網頁內容，直接寫入指定的本地檔案路徑。觸發條件必須同時滿足：(1) 使用者明確提供或要求指定檔案路徑；(2) 意圖是永久保存到磁碟（「存成 xxx.md」、「儲存到 downloads/」、「下載到本地」）。若使用者只是要查看、摘要、分析、爬取內容，一律使用 fetch_page。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"href": map[string]any{
					"type":        "string",
					"description": "要下載的完整網址（需包含 https://）",
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "要儲存的目標檔案路徑（絕對路徑或相對於專案根目錄），例如 ./downloads/page.md",
				},
			},
			"required": []string{"href", "save_to"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Href   string `json:"href"`
				SaveTo string `json:"save_to"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Download(params.Href, params.SaveTo)
		},
	})
}

package browser

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "fetch_page",
		Description: "使用 Chrome 瀏覽器開啟網頁，擷取內容以純文字 Markdown 格式回傳給 agent。此工具不寫入任何檔案，內容僅存在於 agent context。【適用】查詢、摘要、分析、爬取、回答問題等所有「讀取」意圖。【禁止】使用者明確要求將網頁內容寫入本地端檔案（「把這個網頁存成 md」、「下載到本地」、「存到 downloads/」）時，必須改用 download_page；禁止用此工具讀取再自行呼叫 write_file 寫檔。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "要擷取內容的完整網址（需包含 https://）",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "保留與來源網域相同的超連結（用於文件研究任務，需遞迴跟進子頁面時使用）。預設 false。",
				},
			},
			"required": []string{"url"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL       string `json:"url"`
				KeepLinks bool   `json:"keep_links"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Load(params.URL, params.KeepLinks)
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "download_page",
		Description: "將指定 URL 的網頁內容抓取並直接寫入本地端檔案。【觸發條件】必須同時滿足：(1) 有明確的 URL 來源；(2) 使用者意圖是將該 URL 內容永久存到磁碟（「把這個網頁存成 md」、「下載到本地」、「存到 downloads/」）。【禁止】使用者僅查看、摘要、分析網頁內容時，一律用 fetch_page；禁止用於無 URL 的純檔案生成場景（改用 write_file）。未指定 save_to 時，自動存至 ~/Downloads（存在則優先）或 ~/.config/agenvoy/download/<頁面名稱>.md。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"href": map[string]any{
					"type":        "string",
					"description": "要下載的完整網址（需包含 https://）",
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "要儲存的目標檔案路徑。絕對路徑直接使用；相對路徑以 ~/Downloads（存在則優先）或 ~/.config/agenvoy/download/ 為基底。未指定則自動存至該目錄。",
				},
			},
			"required": []string{"href"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Href   string `json:"href"`
				SaveTo string `json:"save_to"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			saveTo := params.SaveTo
			if saveTo != "" {
				abs, err := filesystem.GetAbsPath(filesystem.DownloadDir, saveTo)
				if err != nil {
					return "", fmt.Errorf("filesystem.GetAbsPath: %w", err)
				}
				saveTo = abs
			}
			return Download(params.Href, saveTo)
		},
	})
}

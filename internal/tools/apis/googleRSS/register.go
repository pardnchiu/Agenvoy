package googleRSS

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "fetch_google_rss",
		Description: "透過 Google News RSS 搜尋新聞，返回標題與真實文章連結。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "搜尋關鍵字（支援 Google 搜尋語法，如 'nvidia OR AMD'）",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "時間範圍，可選值：1h / 3h / 6h / 12h / 24h / 7d",
					"enum":        []string{"1h", "3h", "6h", "12h", "24h", "7d"},
				},
				"lang": map[string]any{
					"type":        "string",
					"description": "語言與地區設定，格式 '{country}:{lang}'，預設 'TW:zh-Hant'",
					"default":     "TW:zh-Hant",
				},
			},
			"required": []string{"keyword", "time"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword string `json:"keyword"`
				Time    string `json:"time"`
				Lang    string `json:"lang"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Fetch(params.Keyword, params.Time, params.Lang)
		},
	})
}

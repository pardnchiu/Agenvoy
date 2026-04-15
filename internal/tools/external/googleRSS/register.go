package googleRSS

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var timeRanges = []string{
	"1h", "3h", "6h", "12h", "24h", "7d",
}

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "fetch_google_rss",
		ReadOnly:    true,
		Description: "透過 Google News RSS 搜尋新聞，返回標題、摘要與真實文章連結。【重要】RSS 僅提供標題與片段摘要，不含完整內容。若任務具有研究性質（整理、分析、週報、深度調查等），必須對每一筆返回的連結繼續呼叫 fetch_page 以取得完整文章內容，不得僅依賴 RSS 摘要作為資料來源。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "搜尋關鍵字",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "時間範圍，可選值：1h / 3h / 6h / 12h / 24h / 7d",
					"enum":        timeRanges,
				},
				"language": map[string]any{
					"type":        "string",
					"description": "語言與地區設定，格式 '{country}:{lang}'，預設 'TW:zh-Hant'",
					"default":     "TW:zh-Hant",
				},
			},
			"required": []string{
				"keyword",
				"time_range",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				Query     string `json:"query"`
				Q         string `json:"q"`
				TimeRange string `json:"time_range"`
				Language  string `json:"language"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			if params.Keyword == "" {
				params.Keyword = params.Query
			}
			if params.Keyword == "" {
				params.Keyword = params.Q
			}
			if params.Keyword == "" {
				return "", fmt.Errorf("keyword is required")
			}
			return Fetch(ctx, params.Keyword, params.TimeRange, params.Language)
		},
	})
}

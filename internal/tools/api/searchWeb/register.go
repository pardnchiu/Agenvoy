package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_web",
		Description: "使用 DuckDuckGo 搜尋網路內容，返回標題、網址與摘要列表（JSON 格式）。適合查詢一般資訊、技術文件、產品資料等。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "搜尋關鍵字或問題",
				},
				"range": map[string]any{
					"type":        "string",
					"description": "時間範圍過濾：1h（1小時）、3h（3小時）、6h（6小時）、12h（12小時）、1d（1天）、7d（7天）、1m（1個月）、1y（1年）。不傳則無限制。",
					"enum":        []string{"1h", "3h", "6h", "12h", "1d", "7d", "1m", "1y"},
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
				Range string `json:"range"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Search(ctx, params.Query, TimeRange(params.Range))
		},
	})
}

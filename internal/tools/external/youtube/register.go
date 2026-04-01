package youtube

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "analyze_youtube",
		ReadOnly:    true,
		Description: "分析 YouTube 影片內容，透過 Gemini 進行語音轉文字（STT），返回含時間戳記的完整逐字稿。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "YouTube 影片網址（支援 watch?v=、shorts/、youtu.be/ 格式）",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "自訂分析提示詞，預設為全文逐字稿含時間戳記",
				},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL    string `json:"url"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Fetch(ctx, params.URL, params.Prompt)
		},
	})
}

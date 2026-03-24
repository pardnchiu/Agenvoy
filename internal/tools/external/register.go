package apis

import (
	"context"
	"encoding/json"
	"fmt"

	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "send_http_request",
		Description: "發送 HTTP 請求並返回回應內容。支援 GET、POST（JSON/Form）等方法。適合呼叫 REST API、Webhook 或其他 HTTP 服務。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "完整的 URL（需包含 http:// 或 https://）",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP 方法",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "GET",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "請求標頭（key-value 格式），例如 {\"Authorization\": \"Bearer token\"}",
				},
				"body": map[string]any{
					"type":        "object",
					"description": "請求本體（JSON 格式），適用於 POST/PUT/PATCH",
				},
				"content_type": map[string]any{
					"type":        "string",
					"description": "Content-Type，可選值：json（預設）、form",
					"enum":        []string{"json", "form"},
					"default":     "json",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "請求超時秒數，預設 30 秒，最大 300 秒。一般 REST API 用 30；需要運算的 API（如 AI 生圖、語音合成、影片處理）建議設 120 以上",
					"default":     30,
				},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL         string            `json:"url"`
				Method      string            `json:"method"`
				Headers     map[string]string `json:"headers"`
				Body        map[string]any    `json:"body"`
				ContentType string            `json:"content_type"`
				Timeout     int               `json:"timeout"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return apiAdapter.Send(params.URL, params.Method, params.Headers, params.Body, params.ContentType, params.Timeout)
		},
	})
}

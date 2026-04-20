package external

import (
	"context"
	"encoding/json"
	"fmt"

	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	"github.com/pardnchiu/agenvoy/internal/tools/external/googleRSS"
	"github.com/pardnchiu/agenvoy/internal/tools/external/searchWeb"
	"github.com/pardnchiu/agenvoy/internal/tools/external/yahooFinance"
	"github.com/pardnchiu/agenvoy/internal/tools/external/youtube"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	googleRSS.Register()
	searchWeb.Register()
	yahooFinance.Register()
	youtube.Register()
	toolRegister.Regist(toolRegister.Def{
		Name:       "send_http_request",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Send an HTTP request and return the response content. Supports methods such as GET and POST (JSON/Form).

Suitable for calling REST APIs, webhooks, or other HTTP services.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full URL (must include http:// or https://)",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "GET",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "Request headers (key-value format), e.g. {\"Authorization\": \"Bearer token\"}",
				},
				"body": map[string]any{
					"type":        "object",
					"description": "Request body (JSON format), suitable for POST/PUT/PATCH",
				},
				"content_type": map[string]any{
					"type":        "string",
					"description": "Content-Type, optional values: json (default), form",
					"enum":        []string{"json", "form"},
					"default":     "json",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Request timeout in seconds, default 30 seconds, maximum 300 seconds. Use 30 for general REST APIs; for computational APIs (such as AI image generation, speech synthesis, video processing), 120+ is recommended",
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

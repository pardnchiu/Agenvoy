package external

import (
	"context"
	"encoding/json"
	"fmt"

	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	"github.com/pardnchiu/agenvoy/internal/tools/external/googleRSS"
	"github.com/pardnchiu/agenvoy/internal/tools/external/searchWeb"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	googleRSS.Register()
	searchWeb.Register()
	toolRegister.Regist(toolRegister.Def{
		Name:        "send_http_request",
		AlwaysAllow: false,
		Concurrent:  true,
		Description: "Send an HTTP request (GET/POST/PUT/PATCH/DELETE) to any URL with optional multipart upload. Use when no dedicated api_* tool covers the endpoint; prefer fetch_page for human-readable HTML.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full URL (e.g. 'https://api.example.com/v1/items').",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method.",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "GET",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "Headers (e.g. {\"Authorization\": \"Bearer ...\"}).",
					"default":     map[string]any{},
				},
				"body": map[string]any{
					"type":        "object",
					"description": "Request body (POST/PUT/PATCH). content_type=json/form: flat key-value object. content_type=multipart: {\"fields\":{key:value,...},\"files\":[{\"name\":\"field\",\"path\":\"/abs/path\",\"content_type\":\"application/gzip\"},...]}. File path must be absolute; binary read from disk.",
					"default":     map[string]any{},
				},
				"content_type": map[string]any{
					"type":        "string",
					"description": "Body encoding. multipart for file uploads (binary).",
					"enum":        []string{"json", "form", "multipart"},
					"default":     "json",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout seconds (max 300). Use 120+ for compute-heavy APIs.",
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
			return apiAdapter.Send(ctx, params.URL, params.Method, params.Headers, params.Body, params.ContentType, params.Timeout)
		},
	})
}

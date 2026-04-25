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
		Description: "Send an HTTP request to the specified URL.",
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
					"description": "Request body (POST/PUT/PATCH).",
					"default":     map[string]any{},
				},
				"content_type": map[string]any{
					"type":        "string",
					"description": "Body encoding.",
					"enum":        []string{"json", "form"},
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
			return apiAdapter.Send(params.URL, params.Method, params.Headers, params.Body, params.ContentType, params.Timeout)
		},
	})
}

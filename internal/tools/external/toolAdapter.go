package external

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/internal/tools/external/searchWeb"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	"github.com/pardnchiu/agenvoy/internal/tools/toolcache"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Register() {
	searchWeb.Register()
	registPublicAPIList()
	toolregister.Regist(toolregister.Def{
		Name:        "http_request",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  true,
		Description: `Sends one HTTP request to any URL and returns status, headers and body — GET through DELETE, multipart upload included.
Use for 打 API / 呼叫端點 / POST 一份資料, including a public API found via public_api_list — what an endpoint accepts, returns or rejects is settled here, not by reading a project's source or SDK.
HTML meant to be read → fetch_page; a binary file → download_file; a user-supplied API needing a key → script_* tool via reasoning_guide(tool_generate).`,
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
				"force": map[string]any{
					"type":        "boolean",
					"description": "Skip the cached-result lookup and re-send even if an identical GET was made in the last 30 minutes. Only GET is ever cached. Set true when the user asks to re-check or refresh.",
					"default":     false,
				},
			},
			"required": []string{"url"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL         string            `json:"url"`
				Method      string            `json:"method"`
				Headers     map[string]string `json:"headers"`
				Body        map[string]any    `json:"body"`
				ContentType string            `json:"content_type"`
				Timeout     int               `json:"timeout"`
				Force       bool              `json:"force"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			if !params.Force && toolcache.IsIdempotentRequest(string(args)) {
				if cached, ok := toolcache.FindRecent(e.SessionID, "http_request", string(args)); ok {
					return cached, nil
				}
			}
			return sendHTTPRequest(ctx, params.URL, params.Method, params.Headers, params.Body, params.ContentType, params.Timeout)
		},
	})
}

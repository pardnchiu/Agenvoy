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
		Description: `Sends one HTTP request to a public URL and returns the status, headers and text body; private, loopback and link-local hosts are out of reach.
Use for "call this API", "hit this endpoint", "POST this data", including a public API found via public_api_list — what an endpoint accepts, returns or rejects is settled here, not by reading a project's source or SDK.
HTML meant to be read → fetch_page; a binary file → download_file; a user-supplied API needing a key → script_* tool via reasoning_guide(tool_generate).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full URL with query string, e.g. 'https://api.example.com/v1/items?limit=10'.",
				},
				"method": map[string]any{
					"type":        "string",
					"description": "HTTP method: GET/DELETE send no body; POST/PUT/PATCH send body encoded per content_type.",
					"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
					"default":     "GET",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "Extra headers as string values, e.g. {\"Authorization\": \"Bearer ...\"}; Accept defaults to application/json and Content-Type follows content_type unless overridden here.",
					"default":     map[string]any{},
				},
				"body": map[string]any{
					"type":        "object",
					"description": "Body for POST/PUT/PATCH, ignored for GET/DELETE. json: any object; form: flat key-value object; multipart: {\"fields\":{key:value,...},\"files\":[{\"name\":\"field\",\"path\":\"/abs/path\",\"content_type\":\"application/gzip\"},...]}, each file read from its absolute path.",
					"default":     map[string]any{},
				},
				"content_type": map[string]any{
					"type":        "string",
					"description": "Body encoding: json (application/json), form (x-www-form-urlencoded), multipart (file uploads).",
					"enum":        []string{"json", "form", "multipart"},
					"default":     "json",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds, capped at 300; 120+ for compute-heavy APIs.",
					"default":     30,
				},
				"force": map[string]any{
					"type":        "boolean",
					"description": "true re-sends a GET even if an identical one was made in the last 30 minutes (only GET is cached); use when the user asks to re-check or refresh.",
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

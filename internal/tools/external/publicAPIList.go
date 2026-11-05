package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/configs"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registPublicAPIList() {
	toolregister.Regist(toolregister.Def{
		Name:        "public_api_list",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Public API directory: without category → category names; with category → each keyless API's name, description, HTTPS flag and docs url.
Use for 有沒有公開 API / 找天氣、匯率、地理編碼的免費 API, when no existing tool covers a live-data request.
Picked one → fetch_page its docs url, then http_request; no fitting API → reasoning_guide(tool_generate).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{
					"type":        "string",
					"description": "Category name from the no-category call, case-insensitive (e.g. 'Weather', 'Art & Design'). Omit to list categories first.",
				},
			},
		},
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Category string `json:"category"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			query := url.Values{"type": {"category"}}
			if category := strings.TrimSpace(params.Category); category != "" {
				query = url.Values{"category": {category}}
			}

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			body, _, err := go_pkg_http.GET[string](ctx, http.DefaultClient, configs.ENDPOINT_PUBLIC_API_LIST+"?"+query.Encode(), nil)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return "", utilsmsg.Wrap(utilsmsg.Timeout, "go_pkg_http.GET", "public_api_list", err)
				}
				return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_pkg_http.GET", "public_api_list", err)
			}
			return body, nil
		},
	})
}

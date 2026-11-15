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
		Description: `Public API directory: the category names, or the keyless APIs in one category with their docs urls.
Use for "is there a public API for", "find a free weather / exchange-rate / geocoding API", when no existing tool covers a live-data request.
Picked one → fetch_page its docs url, then http_request; no fitting API → reasoning_guide(tool_generate).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"category": map[string]any{
					"type":        "string",
					"description": "Category name, case-insensitive, e.g. 'Weather', 'Currency Exchange', 'Geocoding', 'Art & Design'; pass it directly when the request clearly fits one (an unknown name returns the full category list), omit only to browse the categories.",
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

			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			fetch := func(query url.Values) (string, error) {
				body, _, err := go_pkg_http.GET[string](ctx, http.DefaultClient, configs.ENDPOINT_PUBLIC_API_LIST+"?"+query.Encode(), nil)
				if err != nil {
					if ctx.Err() == context.DeadlineExceeded {
						return "", utilsmsg.Wrap(utilsmsg.Timeout, "go_pkg_http.GET", "public_api_list", err)
					}
					return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_pkg_http.GET", "public_api_list", err)
				}
				return body, nil
			}

			category := strings.TrimSpace(params.Category)
			if category == "" {
				return fetch(url.Values{"type": {"category"}})
			}

			body, err := fetch(url.Values{"category": {category}})
			if err != nil {
				return "", err
			}
			var result struct {
				Count int `json:"count"`
			}
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Upstream, "json.Unmarshal", "public_api_list", err)
			}
			if result.Count > 0 {
				return body, nil
			}

			raw, err := fetch(url.Values{"type": {"category"}})
			if err != nil {
				return "", err
			}
			var categories struct {
				Data []string `json:"data"`
			}
			if err := json.Unmarshal([]byte(raw), &categories); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Upstream, "json.Unmarshal", "public_api_list", err)
			}
			return "", utilsmsg.New(utilsmsg.NotFound, "category", "%q; available: %s", category, strings.Join(categories.Data, ", "))
		},
	})
}

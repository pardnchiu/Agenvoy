package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var timeRanges = []string{
	"d", "w", "m", "y",
}

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_web",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "[system-default] Web search via DuckDuckGo. Use for named entities, post-cutoff facts, versions, prices, news. Results are snippets. cdp=true forces browser fetch, auto-enabled on 202.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keywords sent to DuckDuckGo Lite (top 10 results, Taiwan locale). Use natural-language keywords, not URLs. Example: 'React 19 release notes'.",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Lookback window: d=past day, w=past week, m=past month, y=past year. Omit for no restriction. Use w for 最近/近期/本週, m for 本月. Default w.",
					"default":     "w",
					"enum":        timeRanges,
				},
				"cdp": map[string]any{
					"type":        "boolean",
					"description": "Force browser-based fetch via Chrome DevTools Protocol instead of HTTP POST. Slower but bypasses HTTP rate-limiting. Automatically enabled on HTTP 202.",
					"default":     false,
				},
			},
			"required": []string{
				"query",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query     string `json:"query"`
				TimeRange string `json:"time_range"`
				CDP       bool   `json:"cdp"`
				// avoid small agent like 4.1 be stupid to call with different parameter name
				Q string `json:"q"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			query := strings.TrimSpace(params.Query)
			if query == "" {
				query = strings.TrimSpace(params.Q)
			}
			if query == "" {
				return "", fmt.Errorf("query is required")
			}

			// avoid small agent like 4.1 be stupid to call with not support value
			timeRange := strings.TrimSpace(params.TimeRange)
			if timeRange != "" && !slices.Contains(timeRanges, params.TimeRange) {
				slog.Warn("invalid time_range, fallback to 'w'")
				params.TimeRange = "w"
			}
			return handler(ctx, query, timeRange, params.CDP)
		},
	})
}

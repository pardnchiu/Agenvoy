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
		Description: "[system-default] Web search. Mandatory for named entities and facts that drift (versions, prices, dates, news). Training knowledge is untrusted for proper nouns and post-cutoff topics. URL given → use fetch_page instead. Results are snippets only — call fetch_page for full content.",
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
			},
			"required": []string{
				"query",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query     string `json:"query"`
				TimeRange string `json:"time_range"`
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
			return handler(ctx, query, timeRange)
		},
	})
}

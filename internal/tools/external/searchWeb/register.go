package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var timeRanges = []string{
	"d", "w", "m", "y",
}

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "search_web",
		ReadOnly: true,
		Description: `
Search the web via DuckDuckGo Lite and return a ranked list of titles, URLs, and snippets.

You MUST cite sources in your response as markdown hyperlinks: [Title](URL).

Suitable for general queries, technical documentation, and product research.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keywords or question",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Time range, available values: d (day) / w (week) / m (month) / y (year), default: w. Omit for no restriction. DuckDuckGo does not support sub-day granularity.",
					"default":     "w",
					"enum":        timeRanges,
				},
			},
			"required": []string{"query"},
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

			if params.Query == "" {
				params.Query = params.Q
			}
			if params.Query == "" {
				return "", fmt.Errorf("query is required")
			}

			// avoid small agent like 4.1 be stupid to call with not support value
			if params.TimeRange == "" || !slices.Contains(timeRanges, params.TimeRange) {
				params.TimeRange = "w"
			}
			return Fetch(ctx, params.Query, params.TimeRange)
		},
	})
}

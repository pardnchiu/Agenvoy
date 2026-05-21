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
		Description: "[system-default] Web search — MANDATORY before answering any 'what/who/why/how is X' question where X is a named entity (project, tool, library, product, company, person, place, event) not yet loaded into this session via prior tool result. Also mandatory for any fact that can drift (releases, versions, prices, dates, specs, APIs, news, schedules). Training memory is untrusted for proper nouns and post-cutoff topics — searching and citing source URLs beats paraphrasing recall; an unfamiliar name is never a reason to skip the search. If the user supplies a URL, use fetch_page instead (never `site:`-wrap a known URL here).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keywords sent to DuckDuckGo Lite (top 10 results, Taiwan locale). Use natural-language keywords, not URLs. Example: 'React 19 release notes'.",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "Lookback window restricting results to the past day/week/month/year. Omit for no restriction; default 'w' biases to recent results.",
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

package googleRSS

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
	"1h", "3h", "6h", "12h", "24h", "7d",
}

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "fetch_google_rss",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Search Google News RSS for news and return the title, summary, and the original article link.

[Important]
RSS only provides titles and short excerpt summaries, not full content.

For research-oriented tasks (compilation, analysis, weekly reports, in-depth investigations, etc.),
you must continue calling fetch_page for each returned link to obtain the full article content,
and must not rely solely on RSS summaries as the source of truth.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Search keywords",
				},
				"time_range": map[string]any{
					"type":        "string",
					"description": "(Optional) Time range, available values: 1h / 3h / 6h / 12h / 24h / 7d",
					"default":     "7d",
					"enum":        timeRanges,
				},
				"ceid": map[string]any{
					"type":        "string",
					"description": "(Optional) Custom Edition ID, format '{country}:{lang}'",
					"default":     "TW:zh-Hant",
				},
			},
			"required": []string{
				"keyword",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword   string `json:"keyword"`
				TimeRange string `json:"time_range"`
				CEID      string `json:"ceid"`
				// avoid small agent like 4.1 be stupid to call with different parameter name
				Query string `json:"query"`
				Q     string `json:"q"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			keyword := strings.TrimSpace(params.Keyword)
			if keyword == "" {
				keyword = strings.TrimSpace(params.Query)
			}
			if keyword == "" {
				keyword = strings.TrimSpace(params.Q)
			}
			if keyword == "" {
				return "", fmt.Errorf("keyword is required")
			}

			// avoid small agent like 4.1 be stupid to call with not support value
			timeRange := strings.TrimSpace(params.TimeRange)
			if timeRange != "" && !slices.Contains(timeRanges, timeRange) {
				slog.Warn("invalid time_range, fallback to '7d'")
				timeRange = "7d"
			}

			var geo, lang string
			ceid := strings.TrimSpace(params.CEID)
			parts := strings.SplitN(ceid, ":", 2)
			if params.CEID == "" || len(parts) != 2 {
				slog.Warn("invalid CEID, fallback to 'TW:zh-Hant'")
				params.CEID = "TW:zh-Hant"
				geo, lang = "TW", "zh-Hant"
			} else {
				geo, lang = parts[0], parts[1]
			}
			return Fetch(ctx, keyword, timeRange, ceid, geo, lang)
		},
	})
}

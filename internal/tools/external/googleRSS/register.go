package googleRSS

import (
	"context"
	"encoding/json"
	"fmt"
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
		Name:     "fetch_google_rss",
		ReadOnly: true,
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
					"description": "Time range, available values: 1h / 3h / 6h / 12h / 24h / 7d, defaut: 7d",
					"default":     "7d",
					"enum":        timeRanges,
				},
				"ceid": map[string]any{
					"type":        "string",
					"description": "Custom Edition ID, format '{country}:{lang}', default: 'TW:zh-Hant'",
					"default":     "TW:zh-Hant",
				},
			},
			"required": []string{
				"keyword",
				"time_range",
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

			if params.Keyword == "" {
				params.Keyword = params.Query
			}
			if params.Keyword == "" {
				params.Keyword = params.Q
			}
			if params.Keyword == "" {
				return "", fmt.Errorf("keyword is required")
			}

			// avoid small agent like 4.1 be stupid to call with not support value
			if params.TimeRange == "" || !slices.Contains(timeRanges, params.TimeRange) {
				params.TimeRange = "7d"
			}

			var geo, lang string
			parts := strings.SplitN(params.CEID, ":", 2)
			if params.CEID == "" || len(parts) != 2 {
				params.CEID = "TW:zh-Hant"
				geo, lang = "TW", "zh-Hant"
			} else {
				geo, lang = parts[0], parts[1]
			}
			return Fetch(ctx, params.Keyword, params.TimeRange, params.CEID, geo, lang)
		},
	})
}

package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_web",
		ReadOnly:    true,
		Description: "Search the web via DuckDuckGo and return a ranked list of titles, URLs, and snippets. You MUST cite sources in your response as markdown hyperlinks: [Title](URL). Suitable for general queries, technical documentation, and product research.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search keywords or question",
				},
				"range": map[string]any{
					"type":        "string",
					"description": "Time range filter: 1d (day), 7d (week), 1m (month), 1y (year). Omit for no restriction. DuckDuckGo does not support sub-day granularity.",
					"enum":        timeRanges,
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query string `json:"query"`
				Range string `json:"range"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			output, err := Search(ctx, params.Query, TimeRange(params.Range))
			if err != nil {
				return "", err
			}

			return formatOutput(params.Query, output), nil
		},
	})
}

func formatOutput(query string, output *SearchOutput) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Web search results for query: %q\n\n", query)

	for _, result := range output.Results {
		fmt.Fprintf(&sb, "%d. [%s](%s)\n", result.Position, result.Title, result.URL)
		if result.Description != "" {
			fmt.Fprintf(&sb, "   %s\n", result.Description)
		}
		sb.WriteByte('\n')
	}

	sb.WriteString("REMINDER: You MUST include the sources above in your response using markdown hyperlinks: [Title](URL)")

	return sb.String()
}

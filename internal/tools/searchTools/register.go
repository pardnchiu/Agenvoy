package searchTools

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
		Name:        "search_tools",
		ReadOnly:    true,
		AlwaysLoad:  true,
		Description: `Search all available tools by keyword and inject matching tools into the current request. Always call this before using any tool to activate its full schema. Supports "select:<name>" for direct selection (comma-separated), space-separated keywords for fuzzy search, and "+" prefix for required terms (e.g. "+file read").`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": `Search query. Use "select:<name>" to activate tools directly (comma-separated for multiple); use space-separated keywords for fuzzy match; prefix "+" marks a required term (e.g. "+file read").`,
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return. Defaults to 5.",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Query      string `json:"query"`
				MaxResults int    `json:"max_results"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.MaxResults <= 0 {
				params.MaxResults = 5
			}

			var matches []result
			if after, ok := strings.CutPrefix(params.Query, "select:"); ok {
				matches = selectByName(after, e.AllTools)
			} else {
				matches = searchByKeyword(params.Query, e.AllTools, params.MaxResults)
			}

			fullSchema := make(map[string]toolTypes.Tool, len(e.AllTools))
			for _, t := range e.AllTools {
				fullSchema[t.Function.Name] = t
			}

			for _, m := range matches {
				if e.ExcludeTools[m.Name] {
					continue
				}
				full, ok := fullSchema[m.Name]
				if !ok {
					continue
				}
				replaced := false
				for i, t := range e.Tools {
					if t.Function.Name == m.Name {
						e.Tools[i] = full
						replaced = true
						break
					}
				}
				if !replaced {
					e.Tools = append(e.Tools, full)
				}
				delete(e.StubTools, m.Name)
			}

			type output struct {
				Injected   []result `json:"injected"`
				Query      string   `json:"query"`
				TotalTools int      `json:"total_tools"`
			}
			out, err := json.Marshal(output{
				Injected:   matches,
				Query:      params.Query,
				TotalTools: len(e.AllTools),
			})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

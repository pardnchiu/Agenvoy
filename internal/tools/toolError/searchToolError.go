package toolError

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registSearchToolError() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "search_tool_errors",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Search past tool-error records for root cause and prior resolution.
Call first when a tool behaves unexpectedly.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "Keyword — tool name, error symptom, or parameter trait.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max results (default 4, max 16).",
					"default":     4,
				},
			},
			"required": []string{
				"keyword",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Keyword string `json:"keyword"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			keyword := strings.TrimSpace(params.Keyword)
			if keyword == "" {
				return "", fmt.Errorf("keyword is required")
			}

			limit := max(1, min(params.Limit, 16))
			return filesystem.SearchErrors(keyword, limit)
		},
	})
}

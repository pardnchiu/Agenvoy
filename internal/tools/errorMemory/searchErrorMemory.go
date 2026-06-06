package errorMemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/memory"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registSearchErrorMemory() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_error_history",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Search past tool-error records for root cause and resolution. Call before 2nd retry when no error hints were injected. Results are authoritative: resolved → apply, failed/abandoned → avoid.",
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
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
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
			return memory.Search(ctx, "", keyword, limit), nil
		},
	})
}

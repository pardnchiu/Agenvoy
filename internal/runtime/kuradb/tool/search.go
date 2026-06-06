package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registSearchRag() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_rag",
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     15 * time.Second,
		Description: "[system-default] Search RAG knowledge base by keyword or semantic mode. Use mode=keyword for precise strings (filenames, terms, names, symbols); use mode=semantic for natural-language queries. If results are sufficient, answer directly without external tools.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"keyword", "semantic"},
					"description": "Search mode: 'keyword' for exact token match, 'semantic' for embedding similarity.",
				},
				"db": map[string]any{
					"type":        "string",
					"description": "Target RAG database name. Call list_rag to discover available databases at runtime.",
				},
				"q": map[string]any{
					"type":        "string",
					"description": "Search query.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max chunks to return (1-100). Invalid values fall back to 10.",
					"default":     10,
				},
			},
			"required": []string{"mode", "db", "q"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode  string `json:"mode"`
				DB    string `json:"db"`
				Q     string `json:"q"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			db := strings.TrimSpace(params.DB)
			q := strings.TrimSpace(params.Q)
			if db == "" {
				return "", fmt.Errorf("db is required")
			}
			if q == "" {
				return "", fmt.Errorf("q is required")
			}
			limit := params.Limit
			if limit < 1 || limit > 100 {
				limit = 10
			}

			var apiPath string
			switch strings.ToLower(strings.TrimSpace(params.Mode)) {
			case "keyword":
				apiPath = "/api/keyword"
			case "semantic":
				apiPath = "/api/semantic"
			default:
				return "", fmt.Errorf("mode must be 'keyword' or 'semantic' (got %q)", params.Mode)
			}

			query := url.Values{}
			query.Set("db", db)
			query.Set("q", q)
			query.Set("limit", strconv.Itoa(limit))
			return kuradbGet(ctx, apiPath, query)
		},
	})
}

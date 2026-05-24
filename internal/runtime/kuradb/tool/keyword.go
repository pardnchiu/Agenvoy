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

func registRagSearchKeyword() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "rag_search_keyword",
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     15 * time.Second,
		Description: `[system-default] Search user's RAG knowledge base by exact token match. Use when query targets a precise string (filename, English term, person name, specific symbol). For natural-language or synonym queries, use rag_search_semantic instead.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"db": map[string]any{
					"type":        "string",
					"description": "Target RAG database name. Call rag_list_db to discover available databases at runtime.",
				},
				"q": map[string]any{
					"type":        "string",
					"description": "Search query. Natural-language input is tokenized into keywords; stopwords are removed.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max chunks to return (1-100). Invalid values fall back to 10.",
					"default":     10,
				},
			},
			"required": []string{"db", "q"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
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
			query := url.Values{}
			query.Set("db", db)
			query.Set("q", q)
			query.Set("limit", strconv.Itoa(limit))
			return kuradbGet(ctx, "/api/keyword", query)
		},
	})
}

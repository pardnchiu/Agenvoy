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

func registRagSearchSemantic() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "rag_search_semantic",
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     15 * time.Second,
		Description: `[system-default] Search user's RAG knowledge base by meaning. Use when query asks about (1) any document/PDF/note content, (2) topics/concepts the user may have ingested, (3) named entities that could be in user's files (e.g. 'X 寫了啥', 'X 詳細資料', '介紹 X'). Prefer this over rag_search_keyword for natural-language and synonym queries.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"db": map[string]any{
					"type":        "string",
					"description": "Target RAG database name. Call rag_list_db to discover available databases at runtime.",
				},
				"q": map[string]any{
					"type":        "string",
					"description": "Natural-language query; semantic similarity is computed against indexed chunk embeddings.",
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
			return kuradbGet(ctx, "/api/semantic", query)
		},
	})
}

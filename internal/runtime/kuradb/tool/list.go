package tool

import (
	"context"
	"encoding/json"
	"time"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRagListDB() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "rag_list_db",
		AlwaysAllow: true,
		Concurrent:  true,
		Timeout:     15 * time.Second,
		Description: `[system-default] List user's RAG knowledge base databases (each db = a group of ingested files). Call this first before any RAG search to discover available db names and decide which to query.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return kuradbGet(ctx, "/api/list", nil)
		},
	})
}

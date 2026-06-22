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
		Name:        "list_rag",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Timeout:     15 * time.Second,
		Description: "List RAG knowledge base databases. Call when the target database name is unknown; skip if the database name is already known (e.g. 'agenvoy').",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return kuradbGet(ctx, "/api/list", nil)
		},
	})
}

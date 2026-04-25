package errorMemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem/errorMemory/toolError"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registReadErrorMemory() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "read_error_memory",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Fetch a prior tool error record by hash.
Use when a tool returns "no data: {hash}" and the full context is needed.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hash": map[string]any{
					"type":        "string",
					"description": "Error hash (8-char hex, e.g. 'a1b2c3d4').",
				},
			},
			"required": []string{
				"hash",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Hash string `json:"hash"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			sessionId := e.SessionID
			if sessionId == "" {
				return "", fmt.Errorf("session not exist")
			}

			hash := strings.TrimSpace(params.Hash)
			if hash == "" {
				return "", fmt.Errorf("hash is required")
			}

			result := toolError.Get(sessionId, hash)
			if result == "" {
				return "not found", nil
			}
			return result, nil
		},
	})
}

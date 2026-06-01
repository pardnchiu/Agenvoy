package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	// window context can not support large content, limit it first
	MaxBytes = 1 << 20 // 1 MiB
)

func (t Tool) getDef(server string, client Client) (toolRegister.Def, bool) {
	params := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	if len(t.InputSchema) > 0 {
		var dic map[string]any
		if err := json.Unmarshal(t.InputSchema, &dic); err == nil && dic != nil {
			params = dic
		}
	}

	description := strings.TrimSpace(t.Description)
	if description == "" {
		description = fmt.Sprintf("MCP tool %q from server %q.", t.Name, server)
	}

	toolName := strings.TrimSpace(t.Name)
	if toolName == "" {
		return toolRegister.Def{}, false
	}
	return toolRegister.Def{
		Name:        "mcp__" + server + "__" + toolName,
		Description: description,
		Parameters:  params,
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var argMap map[string]any
			if len(args) > 0 {
				if err := json.Unmarshal(args, &argMap); err != nil {
					return "", fmt.Errorf("json.Unmarshal: %w", err)
				}
			}

			out, err := client.Call(ctx, toolName, argMap)
			if err != nil {
				return "", err
			}
			if len(out) > MaxBytes {
				out = out[:MaxBytes] + fmt.Sprintf("\n\n[mcp output truncated: %d bytes total, %d kept]", len(out), MaxBytes)
			}
			return out, nil
		},
	}, true
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func (t Tool) getDef(server string, m *MCP) (toolregister.Def, bool) {
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
		return toolregister.Def{}, false
	}
	return toolregister.Def{
		Name:        "mcp__" + server + "__" + toolName,
		Description: description,
		Parameters:  params,
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var argMap map[string]any
			if len(args) > 0 {
				if err := json.Unmarshal(args, &argMap); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			out, err := m.Call(ctx, server, toolName, argMap)
			if err != nil {
				return "", err
			}
			if len(out) > configs.MAX_DOCUMENT_BYTES {
				out = out[:configs.MAX_DOCUMENT_BYTES] + fmt.Sprintf("\n\n[mcp output truncated: %d bytes total, %d kept]", len(out), configs.MAX_DOCUMENT_BYTES)
			}
			return out, nil
		},
	}, true
}

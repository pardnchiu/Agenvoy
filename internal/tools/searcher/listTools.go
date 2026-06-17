package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	systemDefaultMarker = "[system-default]"
)

type Tool struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	SystemDefault bool   `json:"system_default,omitempty"`
}

func isMCPExposed(name string) bool {
	switch {
	case strings.HasPrefix(name, "script_"),
		strings.HasPrefix(name, "api_"),
		strings.HasPrefix(name, "ext_"):
		return true
	}
	return slices.Contains(toolRegister.BuiltinNames(), name)
}

func registListTools() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tools",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `
List available tools by name + one-line description.
Read-only; does not load schemas.
Pass mcp=true to show only MCP-exposed tools (builtin + script_/api_/ext_).
Use search_tools to also activate matching schemas.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mcp": map[string]any{
					"type":        "boolean",
					"description": "When true, only list MCP-exposed tools: builtin + script_/api_/ext_ prefixed. Default false (list all).",
					"default":     false,
				},
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				MCP bool `json:"mcp"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args, &params)
			}

			list := make([]Tool, 0, len(e.AllTools))
			for _, tool := range e.AllTools {
				name := tool.Function.Name
				if params.MCP && !isMCPExposed(name) {
					continue
				}
				list = append(list, Tool{
					Name:          name,
					Description:   tool.Function.Description,
					SystemDefault: strings.HasPrefix(strings.TrimSpace(tool.Function.Description), systemDefaultMarker),
				})
			}

			raw, err := json.Marshal(list)
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

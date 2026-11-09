package toolSearcher

import (
	"context"
	"encoding/json"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRunTool() {
	toolregister.Regist(toolregister.Def{
		Name:      "run_tool",
		SystemUse: true,
		Description: `Calls a tool from the Tools list given before the user input and returns that tool's result.
Every tool listed there by name only goes through here; tools already in this request's tool definitions are called directly.
Before the first call to a name, fetch its schema with find_tools(names=[...]), batching only the names the next step needs.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Exact name from the Tools list.",
				},
				"args": map[string]any{
					"type":        "object",
					"description": "Arguments matching the schema find_tools returned for that name; {} when it takes none.",
					"default":     map[string]any{},
				},
			},
			"required": []string{"name"},
		},
		Handler: func(_ context.Context, _ *tooltypes.Executor, _ json.RawMessage) (string, error) {
			return "", utilsmsg.New(utilsmsg.Internal, "run_tool", "resolved by the exec loop and cannot be dispatched directly")
		},
	})
}

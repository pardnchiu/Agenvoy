package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListTools() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tools",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "List all currently available tools by name + one-line description. Read-only; does not load schemas. Use search_tools to also activate matching schemas.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			type entry struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}

			list := make([]entry, 0, len(e.AllTools))
			for _, t := range e.AllTools {
				list = append(list, entry{
					Name:        t.Function.Name,
					Description: t.Function.Description,
				})
			}

			raw, err := json.Marshal(list)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})

}

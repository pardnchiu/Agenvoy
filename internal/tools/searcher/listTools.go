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
		Name:       "list_tools",
		ReadOnly:   true,
		Concurrent: true,
		Description: "List all currently available built-in and dynamically loaded tools.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			type entry struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}

			list := make([]entry, 0, len(e.Tools))
			for _, t := range e.Tools {
				list = append(list, entry{
					Name:        t.Function.Name,
					Description: t.Function.Description,
				})
			}

			out, err := json.Marshal(list)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})

}

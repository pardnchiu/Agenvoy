package toolSearcher

import (
	"context"
	"encoding/json"
	"fmt"
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

func registListTools() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tools",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `
List all currently available tools by name + one-line description.
Read-only; does not load schemas.
Use search_tools to also activate matching schemas.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			list := make([]Tool, 0, len(e.AllTools))
			for _, tool := range e.AllTools {
				list = append(list, Tool{
					Name:          tool.Function.Name,
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

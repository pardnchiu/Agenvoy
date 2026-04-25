package calculator

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "calculate",
		ReadOnly:    true,
		Concurrent:  true,
		Description: "Evaluate a mathematical expression and return the exact result.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{
					"type":        "string",
					"description": "Mathematical expression, for example '(100 + 200) * 3', '10 % 3', '2 ^ 10', 'sqrt(2)', or 'pow(2, 10)'.",
				},
			},
			"required": []string{"expression"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Expression string `json:"expression"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return Calc(params.Expression)
		},
	})
}

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
		Description: "執行數學運算，返回精確結果。支援四則運算（+、-、*、/）、取模（%）、括號、冪次（用 ^ 符號）及數學函式（sqrt、abs、ceil、floor、round、log、log2、log10、sin、cos、tan、pow）。pow 需傳入兩個引數：pow(base, exp)。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{
					"type":        "string",
					"description": "數學表達式，例如 '(100 + 200) * 3'、'10 % 3'、'2 ^ 10'、'sqrt(2)'、'pow(2, 10)'、'1000000 * 0.07 / 12'",
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

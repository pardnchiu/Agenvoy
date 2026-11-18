// * last modified: 26/11/19
package calculate

import (
	"context"
	"encoding/json"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Register() {
	toolregister.Regist(toolregister.Def{
		Name:        "calculate",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Evaluates arithmetic and math functions on literal numbers, returning each result; no units, variables or dates.
Use for "calculate", "how much is that in total", "convert this at rate X", and whenever a number would otherwise be worked out in the head.
Computes only, never looks up: a rate, price or live figure comes from its own tool first and is passed in as a literal.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expressions": map[string]any{
					"type":        "array",
					"description": "Independent expressions, e.g. '(100 + 200) * 3', '1250 * 32.4', '2 ^ 10'; decimal literals only. Operators + - * / % ^; ^ is power, binds tightest, right-associative (-2^2 = -4, 2^3^2 = 512). Integers stay exact; any float operand makes the result float64 (~16 significant digits); integer / integer truncates (7 / 2 = 3), so write 7.0 / 2 for a fraction. Functions: sqrt, abs, ceil, floor, round, log (natural), log2, log10, sin, cos, tan (radians), pow(x, y), min(...), max(...).",
					"items":       map[string]any{"type": "string"},
					"minItems":    1,
				},
			},
			"required": []string{"expressions"},
		},
		Handler: func(_ context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Expressions []string `json:"expressions"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if len(params.Expressions) == 0 {
				return "", utilsmsg.New(utilsmsg.Invalid, "expressions", "required")
			}

			out := make(map[string]string, len(params.Expressions))
			for _, expr := range params.Expressions {
				result, err := Calc(expr)
				if err != nil {
					result = "error: " + err.Error()
				}
				out[expr] = result
			}

			raw, err := utils.MarshalPlain(out)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "utils.MarshalPlain", "", err)
			}
			return string(raw), nil
		},
	})
}

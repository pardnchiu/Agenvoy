package variant

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registTestHook() {
	toolregister.Regist(toolregister.Def{
		Name: "test_hook",
		Description: `Runs the installed hook (.py over .js) once the same way Agenvoy does, with JSON on stdin, and returns its success and debug / error, or why the run failed (crash, timeout, a last stdout line that is not the required JSON).
Use after every write_hook write or patch, before the hook is relied on.
It fails or returns the wrong success → fix with write_hook(mode=patch) and test again, repeating until it is right; then stop.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"enum":        agentsexechook.Names(),
					"description": "The hook state to run.",
				},
				"input": map[string]any{
					"type":        "string",
					"description": "JSON object fed on stdin, using the fields read_hook's demo reads, with values that exercise the new rule, e.g. '{\"input\":\"token=abcdefghijklmnop1234\"}'.",
					"default":     "{}",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name  string `json:"name"`
				Input string `json:"input"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			name := strings.TrimSpace(params.Name)
			if !slices.Contains(agentsexechook.Names(), name) {
				return "", utilsmsg.New(utilsmsg.Invalid, "name", "unknown hook %q; available: %s", name, strings.Join(agentsexechook.Names(), ", "))
			}
			input := strings.TrimSpace(params.Input)
			if input == "" {
				input = "{}"
			}
			if !json.Valid([]byte(input)) {
				return "", utilsmsg.New(utilsmsg.Invalid, "input", "not valid JSON: %s", input)
			}

			isPass, message, err := agentsexechook.Test(ctx, name, json.RawMessage(input))
			if err != nil {
				return "", err
			}
			if isPass {
				return "success: 1\ndebug: " + message, nil
			}
			return "success: 0\nerror: " + message, nil
		},
	})
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"

	_ "github.com/pardnchiu/agenvoy/internal/tools/calculator"
	_ "github.com/pardnchiu/agenvoy/internal/tools/external"
	_ "github.com/pardnchiu/agenvoy/internal/tools/external/googleRSS"
	_ "github.com/pardnchiu/agenvoy/internal/tools/external/searchWeb"
	_ "github.com/pardnchiu/agenvoy/internal/tools/external/yahooFinance"
	_ "github.com/pardnchiu/agenvoy/internal/tools/external/youtube"
	_ "github.com/pardnchiu/agenvoy/internal/tools/externalAgent"
	_ "github.com/pardnchiu/agenvoy/internal/tools/fetchPage"
	_ "github.com/pardnchiu/agenvoy/internal/tools/file"
	_ "github.com/pardnchiu/agenvoy/internal/tools/git"
	_ "github.com/pardnchiu/agenvoy/internal/tools/schedulerTools"
	_ "github.com/pardnchiu/agenvoy/internal/tools/searchTools"
	_ "github.com/pardnchiu/agenvoy/internal/tools/skillTool"
)

func init() {
	toolRegister.RegistGroup("api_", func(_ context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
		if e.APIToolbox == nil || !e.APIToolbox.IsExist(name) {
			return "", fmt.Errorf("not exist: %s", name)
		}

		var params map[string]any
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return e.APIToolbox.Execute(name, params)
	})

	toolRegister.RegistGroup("script_", func(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
		if e.ScriptToolbox == nil || !e.ScriptToolbox.IsExist(name) {
			return "", fmt.Errorf("not exist: %s", name)
		}
		return e.ScriptToolbox.Execute(ctx, name, args, e.WorkDir)
	})

	registRunCommand()

	toolRegister.Regist(toolRegister.Def{
		Name:        "list_tools",
		ReadOnly:    true,
		Description: "List all currently available tools, including built-in tools and dynamically loaded API tools (prefixed with api_*). Returns each tool's name and description.",
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

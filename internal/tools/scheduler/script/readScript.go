package scriptTools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registReadScript() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "read_script",
		ReadOnly:    true,
		Description: "Read the contents of a scheduler script by filename.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Scheduler script filename including extension, for example 'notify_1741569300.sh'.",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}
			data, err := os.ReadFile(filepath.Join(filesystem.ScriptsDir, params.Name))
			if err != nil {
				return "", fmt.Errorf("os.ReadFile: %w", err)
			}
			return string(data), nil
		},
	})

}

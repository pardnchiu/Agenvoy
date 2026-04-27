package scriptTools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdateScript() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "update_script",
		Description: "Overwrite an existing scheduler script without changing its filename.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Scheduler script filename to overwrite, including extension.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "New script content.",
				},
			},
			"required": []string{"name", "content"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if filepath.Base(params.Name) != params.Name {
				return "", fmt.Errorf("must not contain path separator")
			}
			if params.Content == "" {
				return "", fmt.Errorf("content is required")
			}
			if err := go_utils_filesystem.WriteFile(filepath.Join(filesystem.ScriptsDir, params.Name), params.Content, 0755); err != nil {
				return "", fmt.Errorf("go_utils_filesystem.WriteFile: %w", err)
			}
			return fmt.Sprintf("script updated: %s", params.Name), nil
		},
	})
}

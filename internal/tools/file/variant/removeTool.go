package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveTool() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_tool",
		Description: `
Remove an existing script tool directory under ~/.config/agenvoy/tools/script/<toolname>/.
Use when a tool is no longer needed or must be rebuilt from scratch.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Tool name in snake_case without 'script_' prefix.",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}

			baseDir, err := go_pkg_filesystem.AbsPath("", scriptToolBaseDir, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AbsPath [%s]: %w", scriptToolBaseDir, err)
			}

			dir := filepath.Join(baseDir, name)
			if !go_pkg_filesystem_reader.IsDir(dir) {
				return "", fmt.Errorf("tool %q does not exist", name)
			}

			if err := os.RemoveAll(dir); err != nil {
				return "", fmt.Errorf("os.RemoveAll [%s]: %w", dir, err)
			}

			return fmt.Sprintf("removed: %s", dir), nil
		},
	})
}

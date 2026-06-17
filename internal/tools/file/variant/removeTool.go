package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveTool() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_tool",
		Description: `
Move a script tool directory to ~/.config/agenvoy/tools/script/.Trash/.
Use when a tool is obsolete or must be rebuilt; recoverable via git_rollback.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Snake_case name without 'script_' prefix (e.g. 'ip_geolocation_lookup').",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}

			dir := filepath.Join(filesystem.ScriptToolsDir, name)
			if !go_pkg_filesystem_reader.IsDir(dir) {
				return "", fmt.Errorf("tool %q does not exist", name)
			}

			_, err := filesystem.TrashDir(dir, filesystem.ScriptToolTrashDir, name)
			if err != nil {
				return "", err
			}

			filesystem.GitAutoCommit(ctx, filesystem.GitTools, "trash", name)
			return fmt.Sprintf("trashed: %s", dir), nil
		},
	})
}

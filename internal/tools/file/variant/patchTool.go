package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchTool() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_tool",
		AlwaysAllow: true,
		Description: `
Overwrite a single file (tool.json or script.py) of an existing script tool.
Use when fixing a broken tool after a failed test run.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Existing tool name in snake_case without 'script_' prefix.",
				},
				"tag": map[string]any{
					"type":        "string",
					"description": "Which file to patch.",
					"enum":        []string{"json", "script"},
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full replacement content for the target file.",
				},
			},
			"required": []string{"name", "tag", "content"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name    string `json:"name"`
				Tag     string `json:"tag"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			if strings.TrimSpace(params.Content) == "" {
				return "", fmt.Errorf("content is required")
			}

			var filename string
			switch params.Tag {
			case "json":
				filename = "tool.json"
			case "script":
				filename = "script.py"
			default:
				return "", fmt.Errorf("tag must be 'json' or 'script', got %q", params.Tag)
			}

			baseDir, err := go_pkg_filesystem.AbsPath("", scriptToolBaseDir, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AbsPath [%s]: %w", scriptToolBaseDir, err)
			}

			dir := filepath.Join(baseDir, name)
			if !go_pkg_filesystem_reader.IsDir(dir) {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem IsDir [%s]: %q does not exist", dir, name)
			}

			target := filepath.Join(dir, filename)
			if err := go_pkg_filesystem.WriteFile(target, params.Content, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile [%s]: %w", filename, err)
			}

			return fmt.Sprintf("patched: %s", target), nil
		},
	})
}

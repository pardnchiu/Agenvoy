package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteTool() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "write_tool",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `
Create or overwrite a tool file. tag=json|script writes under script tool directory; tag=api writes a single JSON to the API tool directory.
Use in Capability Gap flow; patch_tool for string replacement, test_tool to verify.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Snake_case name without 'script_' prefix; runtime adds it (e.g. 'ip_geolocation_lookup').",
				},
				"tag": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "script", "api"},
					"description": "Target file. 'json' = tool.json (schema), 'script' = script.py (runtime), 'api' = <name>.json (API tool definition).",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full file content to write. Must be complete, not a diff.",
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
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			if strings.TrimSpace(params.Content) == "" {
				return "", fmt.Errorf("content is required")
			}

			var target string
			switch params.Tag {
			case "json":
				target = filepath.Join(filesystem.ScriptToolsDir, name, "tool.json")
			case "script":
				target = filepath.Join(filesystem.ScriptToolsDir, name, "script.py")
			case "api":
				target = filepath.Join(filesystem.APIToolsDir, name+".json")
			default:
				return "", fmt.Errorf("tag must be 'json', 'script', or 'api', got %q", params.Tag)
			}
			if err := go_pkg_filesystem.WriteFile(target, params.Content, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/agenvoy/internal/filesystem: WriteFile [%s]: %w", target, err)
			}

			filesystem.GitAutoCommit(ctx, filesystem.GitTools, "add", name)
			return fmt.Sprintf("created: %s", target), nil
		},
	})
}

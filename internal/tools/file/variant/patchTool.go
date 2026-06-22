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

func registPatchTool() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_tool",
		AlwaysAllow: true,
		Description: `
Replace an exact string match inside a tool file (tool.json, script.py, or API tool JSON).
Use to fix a broken tool after test_tool failure; write_tool for full rewrite.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Snake_case name without 'script_' prefix (e.g. 'ip_geolocation_lookup').",
				},
				"tag": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "script", "api"},
					"description": "Target file. 'json' = tool.json (schema fix), 'script' = script.py (runtime fix), 'api' = <name>.json (API tool fix).",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact string to replace. Must be unique in the target file.",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement string. Empty string deletes old_string.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace all occurrences. Defaults to false.",
					"default":     false,
				},
			},
			"required": []string{"name", "tag", "old_string", "new_string"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name       string `json:"name"`
				Tag        string `json:"tag"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("encoding/json: Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			if params.OldString == "" {
				return "", fmt.Errorf("old_string is required")
			}
			if params.OldString == params.NewString {
				return "", fmt.Errorf("no edit needed")
			}

			var target string
			switch params.Tag {
			case "json":
				dir := filepath.Join(filesystem.ScriptToolsDir, name)
				if !go_pkg_filesystem_reader.IsDir(dir) {
					return "", fmt.Errorf("tool %q does not exist", name)
				}
				target = filepath.Join(dir, "tool.json")
			case "script":
				dir := filepath.Join(filesystem.ScriptToolsDir, name)
				if !go_pkg_filesystem_reader.IsDir(dir) {
					return "", fmt.Errorf("tool %q does not exist", name)
				}
				target = filepath.Join(dir, "script.py")
			case "api":
				target = filepath.Join(filesystem.APIToolsDir, name+".json")
				if !go_pkg_filesystem_reader.Exists(target) {
					return "", fmt.Errorf("api tool %q does not exist", name)
				}
			default:
				return "", fmt.Errorf("tag must be 'json', 'script', or 'api', got %q", params.Tag)
			}
			if err := patch(target, params.OldString, params.NewString, params.ReplaceAll); err != nil {
				return "", err
			}

			filesystem.GitAutoCommit(ctx, filesystem.GitTools, "update", name)
			return fmt.Sprintf("updated: %s", target), nil
		},
	})
}

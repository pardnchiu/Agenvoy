package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registTestTool() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "test_tool",
		AlwaysAllow: true,
		Description: `
Run a script tool's script.py with JSON input inside sandbox.
Use after write_tool or patch_tool to verify before production use.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Snake_case name without 'script_' prefix (e.g. 'ip_geolocation_lookup').",
				},
				"input": map[string]any{
					"type":        "string",
					"description": "JSON string fed as stdin to script.py (e.g. '{\"ip\":\"8.8.8.8\"}').",
					"default":     "{}",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name  string `json:"name"`
				Input string `json:"input"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}

			input := strings.TrimSpace(params.Input)
			if input == "" {
				input = "{}"
			}

			scriptPath := filepath.Join(filesystem.ScriptToolsDir, name, "script.py")
			if !go_pkg_filesystem_reader.Exists(scriptPath) {
				return "", fmt.Errorf("script not found: %s", scriptPath)
			}

			execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			cmd, err := go_pkg_sandbox.Wrap(execCtx, "python3", []string{scriptPath}, filesystem.ScriptToolsDir, nil)
			if err != nil {
				return "", fmt.Errorf("sandbox.Wrap: %w", err)
			}

			cmd.Stdin = strings.NewReader(input)

			var stdout, stderr strings.Builder
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err != nil {
				if stderr.Len() > 0 {
					return "", fmt.Errorf("script error: %s", strings.TrimSpace(stderr.String()))
				}
				return "", fmt.Errorf("exec: %w", err)
			}

			return strings.TrimSpace(stdout.String()), nil
		},
	})
}

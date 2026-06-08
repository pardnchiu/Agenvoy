package variant

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const scriptToolBaseDir = "~/.config/agenvoy/tools/script"

func registGenerateTool() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "generate_tool",
		AlwaysAllow: true,
		Description: `
Create a new script tool (tool.json + script.py) under ~/.config/agenvoy/tools/script/<toolname>/.
Use when building a reusable tool via the Capability Gap flow.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Tool name in snake_case without 'script_' prefix (runtime adds it). Example: 'ip_geolocation_lookup'.",
				},
				"json": map[string]any{
					"type":        "string",
					"description": "Full content of tool.json (JSON string with name, description, always_allow, parameters).",
				},
				"script": map[string]any{
					"type":        "string",
					"description": "Full content of script.py (Python script that reads stdin JSON, calls API, prints JSON to stdout).",
				},
			},
			"required": []string{"name", "json", "script"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name   string `json:"name"`
				JSON   string `json:"json"`
				Script string `json:"script"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			name := strings.TrimSpace(params.Name)
			if name == "" {
				return "", fmt.Errorf("name is required")
			}
			if strings.TrimSpace(params.JSON) == "" {
				return "", fmt.Errorf("json is required")
			}
			if strings.TrimSpace(params.Script) == "" {
				return "", fmt.Errorf("script is required")
			}

			baseDir, err := go_pkg_filesystem.AbsPath("", scriptToolBaseDir, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AbsPath [%s]: %w", scriptToolBaseDir, err)
			}

			dir := filepath.Join(baseDir, name)
			if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir [%s]: %w", dir, err)
			}

			jsonPath := filepath.Join(dir, "tool.json")
			if err := go_pkg_filesystem.WriteFile(jsonPath, params.JSON, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile [%s]: %w", jsonPath, err)
			}

			scriptPath := filepath.Join(dir, "script.py")
			if err := go_pkg_filesystem.WriteFile(scriptPath, params.Script, 0644); err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile [%s]: %w", scriptPath, err)
			}

			return fmt.Sprintf("tool created: %s\n  %s\n  %s", dir, jsonPath, scriptPath), nil
		},
	})
}

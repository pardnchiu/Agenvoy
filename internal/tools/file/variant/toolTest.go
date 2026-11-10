package variant

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registTestTool() {
	toolregister.Regist(toolregister.Def{
		Name:        "test_tool",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Runs a script tool's script.py in the sandbox (60s limit) with JSON on stdin and returns its stdout, or its stderr as the error; tool.json is not checked.
Use for "test this tool", "check the script works", and after every edit_tool write or patch before the tool is used for real.
It fails → edit_tool(mode=patch), then test again; checking tool.json or editing the tool → edit_tool.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Tool directory name in snake_case without the 'script_' prefix, as given to edit_tool, e.g. 'ip_geolocation_lookup'.",
				},
				"input": map[string]any{
					"type":        "string",
					"description": "JSON string fed to script.py on stdin, shaped like the tool's parameters, e.g. '{\"ip\":\"8.8.8.8\"}'.",
					"default":     "{}",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Name  string `json:"name"`
				Input string `json:"input"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return testToolScript(ctx, params.Name, params.Input)
		},
	})
}

func testToolScript(ctx context.Context, name, input string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "name", "required")
	}

	input = strings.TrimSpace(input)
	if input == "" {
		input = "{}"
	}

	scriptPath, err := toolTarget(name, "script", false)
	if err != nil {
		return "", err
	}
	if !go_pkg_filesystem_reader.Exists(scriptPath) {
		return "", utilsmsg.New(utilsmsg.NotFound, "script", "%s", scriptPath)
	}

	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd, err := go_pkg_sandbox.Wrap(execCtx, "python3", []string{scriptPath}, filesystem.ScriptToolsDir, nil)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err)
	}

	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := go_pkg_sandbox.Run(cmd); err != nil {
		if stderr.Len() > 0 {
			return "", utilsmsg.New(utilsmsg.Internal, "test_tool", "script error: %s", strings.TrimSpace(stderr.String()))
		}
		return "", utilsmsg.Wrap(utilsmsg.Internal, "exec", "", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

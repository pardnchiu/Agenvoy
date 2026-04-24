package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	go_utils_sandbox "github.com/pardnchiu/go-utils/sandbox"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRunCommand() {
	toolRegister.Regist(toolRegister.Def{
		Name: "run_command",
		Description: `
Run a binary with argv; returns combined stdout/stderr.
Executes in the work directory — do NOT prepend 'cd'; use absolute paths.
For pipes/redirects/shell expansion, pass argv=['sh','-c','<full shell command>'].`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"argv": map[string]any{
					"type":        "array",
					"description": "Command as argv array. e.g. ['git','status'] or ['python3','script.py','--name','value with spaces']. For shell features use ['sh','-c','cmd | pipe'].",
					"items":       map[string]any{"type": "string"},
					"minItems":    1,
				},
			},
			"required": []string{"argv"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Argv []string `json:"argv"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return runCommand(ctx, e, params.Argv)
		},
	})
}

func runCommand(ctx context.Context, e *toolTypes.Executor, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("run_command requires a non-empty 'argv' array, e.g. [\"git\", \"status\"]")
	}

	joined := strings.Join(argv, " ")
	for _, dir := range DeniedConfig.Dirs {
		if strings.Contains(joined, "/"+dir+"/") || strings.Contains(joined, "/"+dir) || strings.Contains(joined, dir+"/") {
			return "", fmt.Errorf("access denied: %s", dir)
		}
	}
	for _, f := range DeniedConfig.Files {
		if strings.Contains(joined, f) {
			return "", fmt.Errorf("access denied: %s", f)
		}
	}

	binary := filepath.Base(argv[0])

	if binary == "sh" && len(argv) >= 3 && argv[1] == "-c" {
		inner := strings.Fields(argv[2])
		if len(inner) == 0 {
			return "", fmt.Errorf("sh -c requires a non-empty command string")
		}
		if !e.AllowedCommand[filepath.Base(inner[0])] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", inner[0])
		}
	} else {
		if !e.AllowedCommand[binary] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", binary)
		}
		if binary == "rm" {
			return moveToTrash(ctx, e, argv[1:])
		}
	}

	// TODO: need to change to dynamic timeout based on command complexity
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	cmd, err := go_utils_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, nil)
	if err != nil {
		return "", fmt.Errorf("sandbox.Wrap: %w", err)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s", string(output), err.Error()), nil
	}

	return string(output), nil
}

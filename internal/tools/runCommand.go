package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
Run a shell command; returns combined stdout/stderr.

Executes in the work directory — do NOT prefix with 'cd'; it will be rejected.

Resolve the exact command before calling; no placeholders.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command (e.g. 'git status', 'ls -la /tmp'). No 'cd' prefix; no placeholders.",
				},
			},
			"required": []string{"command"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return runCommand(ctx, e, params.Command)
		},
	})
}

func runCommand(ctx context.Context, e *toolTypes.Executor, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("run_command requires a non-empty 'command' argument. Call this tool again with the exact shell command to execute, e.g. {\"command\": \"git diff --cached\"}")
	}

	for _, dir := range DeniedConfig.Dirs {
		if strings.Contains(command, "/"+dir+"/") || strings.Contains(command, "/"+dir) || strings.Contains(command, dir+"/") {
			return "", fmt.Errorf("access denied: %s", dir)
		}
	}
	for _, f := range DeniedConfig.Files {
		if strings.Contains(command, f) {
			return "", fmt.Errorf("access denied: %s", f)
		}
	}

	// * template allow all for testing
	// if disallowed.MatchString(command) {
	// 	return "", fmt.Errorf("failed to run command: disallowed characters")
	// }

	hasShellOps := strings.ContainsAny(command, "|><&")

	var binary string
	var args []string

	if hasShellOps {
		binary = "sh"
		args = []string{"-c", command}

		firstCmd := strings.Fields(command)[0]
		if !e.AllowedCommand[filepath.Base(firstCmd)] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", firstCmd)
		}
	} else {
		args = strings.Fields(command)
		binary = filepath.Base(args[0])

		if !e.AllowedCommand[binary] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", binary)
		}

		if binary == "rm" {
			return moveToTrash(ctx, e, args[1:])
		}
	}

	// TODO: need to change to dynamic timeout based on command complexity
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var (
		cmd *exec.Cmd
		err error
	)
	if hasShellOps {
		cmd, err = go_utils_sandbox.Wrap(ctx, binary, args, e.WorkDir, nil)
	} else {
		cmd, err = go_utils_sandbox.Wrap(ctx, args[0], args[1:], e.WorkDir, nil)
	}
	if err != nil {
		return "", fmt.Errorf("sandbox.Wrap: %w", err)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s", string(output), err.Error()), nil
	}

	return string(output), nil
}

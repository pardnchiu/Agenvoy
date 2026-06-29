package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/sudo"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var SudoStreamHook func(line string)

func registRunCommand() {
	toolRegister.Regist(toolRegister.Def{
		Name: "run_command",
		Description: `
Run a binary with argv; returns combined stdout/stderr.
Executes in the work directory. Use ['cd', '<path>'] to change the work directory for subsequent commands; the path is verified before switching.
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

	elevated := sudo.IsActive()

	joined := strings.Join(argv, " ")

	if elevated {
		sudo.Refresh()
		if blocked, hit := sudo.HitFloor(joined); hit {
			return "", fmt.Errorf("access denied (floor): %s", blocked)
		}
		slog.Warn("sudo exec",
			slog.String("session", e.SessionID),
			slog.String("command", joined))
	} else {
		for _, dir := range filesystem.DeniedMap.Dirs {
			if strings.Contains(joined, "/"+dir+"/") || strings.Contains(joined, "/"+dir) || strings.Contains(joined, dir+"/") {
				return "", fmt.Errorf("access denied: %s", dir)
			}
		}
		for _, f := range filesystem.DeniedMap.Files {
			if strings.Contains(joined, f) {
				return "", fmt.Errorf("access denied: %s", f)
			}
		}
	}

	binary := filepath.Base(argv[0])

	if (binary == "sh" || binary == "bash") && len(argv) >= 3 && argv[1] == "-c" {
		if !elevated && !e.AllowedCommand[binary] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", binary)
		}
		if strings.TrimSpace(argv[2]) == "" {
			return "", fmt.Errorf("%s -c requires a non-empty command string", binary)
		}
		if elevated {
			if err := validateShellScriptFloor(argv[2]); err != nil {
				return "", err
			}
		} else {
			if err := validateShellScript(argv[2], e.AllowedCommand); err != nil {
				return "", err
			}
		}
	} else {
		if binary == "cd" {
			return changeWorkDir(e, argv[1:])
		}
		if !elevated && !e.AllowedCommand[binary] {
			return "", fmt.Errorf("failed to run command: %s is not allowed", binary)
		}
		if binary == "rm" {
			return moveToTrash(ctx, e, argv[1:])
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var sandboxOpt *go_pkg_sandbox.Option
	if elevated {
		sandboxOpt = &go_pkg_sandbox.Option{AllowAll: true}
	}

	cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
	if err != nil {
		return "", fmt.Errorf("sandbox.Wrap: %w", err)
	}

	if elevated && SudoStreamHook != nil {
		return runCommandStreaming(cmd)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s", string(output), err.Error()), nil
	}

	return string(output), nil
}

func runCommandStreaming(cmd *exec.Cmd) (string, error) {
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	var sb strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			sb.WriteString(line)
			sb.WriteByte('\n')
			if hook := SudoStreamHook; hook != nil {
				hook(line)
			}
		}
	}()

	err := cmd.Run()
	pw.Close()
	<-done

	if err != nil {
		return fmt.Sprintf("%s\nError: %s", sb.String(), err.Error()), nil
	}

	return sb.String(), nil
}

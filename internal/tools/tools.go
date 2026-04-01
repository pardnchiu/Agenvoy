package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/sandbox"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type deniedConfig struct {
	Dirs       []string `json:"dirs"`
	Files      []string `json:"files"`
	Prefixes   []string `json:"prefixes"`
	Extensions []string `json:"extensions"`
}

var DeniedConfig = func() deniedConfig {
	var cfg deniedConfig
	if err := json.Unmarshal(configs.DeniedMap, &cfg); err != nil {
		slog.Warn("json.Unmarshal",
			slog.String("error", err.Error()))
	}
	return cfg
}()

var (
// * template allow all for testing
// disallowed = regexp.MustCompile(`[;&|` + "`" + `$(){}!<>\\]`)
)

func runCommand(ctx context.Context, e *toolTypes.Executor, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("failed to run command: command is empty")
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
		wrappedBin  string
		wrappedArgs []string
		err         error
	)
	if hasShellOps {
		wrappedBin, wrappedArgs, err = sandbox.Wrap(binary, args, e.WorkDir)
	} else {
		wrappedBin, wrappedArgs, err = sandbox.Wrap(args[0], args[1:], e.WorkDir)
	}
	if err != nil {
		return "", fmt.Errorf("sandbox.Wrap: %w", err)
	}

	cmd := exec.CommandContext(ctx, wrappedBin, wrappedArgs...)
	cmd.Dir = e.WorkDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s\nError: %s", string(output), err.Error()), nil
	}

	return string(output), nil
}

func moveToTrash(ctx context.Context, e *toolTypes.Executor, args []string) (string, error) {
	trashPath := filepath.Join(e.WorkDir, ".Trash")
	if err := os.MkdirAll(trashPath, 0755); err != nil {
		return "", fmt.Errorf("os.MkdirAll .Trash: %w", err)
	}

	var moved []string
	for _, arg := range args {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("moveToTrash cancelled: %w", err)
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		src := filepath.Join(e.WorkDir, filepath.Clean(arg))
		name := filepath.Base(arg)
		dst := filepath.Join(trashPath, name)

		if _, err := os.Stat(dst); err == nil {
			ext := filepath.Ext(name)
			dst = filepath.Join(trashPath, fmt.Sprintf("%s_%s%s",
				strings.TrimSuffix(name, ext),
				time.Now().Format("20060102_150405"),
				ext))
		}

		if err := os.Rename(src, dst); err == nil {
			moved = append(moved, arg)
		}
	}
	return fmt.Sprintf("Successfully moved to .Trash: %s", strings.Join(moved, ", ")), nil
}

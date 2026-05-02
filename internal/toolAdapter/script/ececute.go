package scriptAdapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

func (t *Translator) Execute(ctx context.Context, name string, args json.RawMessage, workDir string) (string, error) {
	key := strings.TrimPrefix(name, "script_")
	data, ok := t.scripts[key]
	if !ok {
		return "", fmt.Errorf("script tool not found: %s", key)
	}

	runtime := runtimeMap[data.language]
	if runtime == "" {
		return "", fmt.Errorf("runtime unsupported: %s", data.language)
	}

	input := string(args)
	if input == "" || input == "null" {
		input = "{}"
	}

	// * max 5min with every 30s check
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd, err := go_pkg_sandbox.Wrap(execCtx, runtime, []string{data.scriptPath}, workDir, nil)
	if err != nil {
		return "", fmt.Errorf("sandbox.Wrap: %w", err)
	}

	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cmd.Start: %w", err)
	}

	start := time.Now()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	for {
		select {
		case err := <-done:
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) || stderr.Len() > 0 {
					return "", fmt.Errorf("script error: %s", strings.TrimSpace(stderr.String()))
				}
				if execCtx.Err() == context.DeadlineExceeded {
					return "", fmt.Errorf("execution timeout: 5m")
				}
				return "", fmt.Errorf("exec: %w", err)
			}
			return strings.TrimSpace(stdout.String()), nil

		case <-ticker.C:
			slog.Info("running",
				slog.String("name", key),
				slog.String("elapsed", fmt.Sprintf("%ds/300s", int(time.Since(start).Seconds()))))
		}
	}
}

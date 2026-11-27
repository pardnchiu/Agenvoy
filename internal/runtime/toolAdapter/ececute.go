package toolAdapter

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

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func (t *Translator) Execute(ctx context.Context, name string, args json.RawMessage, workDir string) (string, error) {
	key := strings.TrimPrefix(name, t.prefix)
	data, ok := t.scripts[key]
	if !ok {
		return "", utilsmsg.New(utilsmsg.NotFound, "script tool", "%s", key)
	}

	runtime := runtimeMap[data.language]
	if runtime == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "runtime", "unsupported: %s", data.language)
	}

	input := string(args)
	if input == "" || input == "null" {
		input = "{}"
	}

	timeout := configs.DEFAULT_TOOL_TIMEOUT
	if data.Doc.Timeout > 0 {
		timeout = time.Duration(data.Doc.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd, err := go_pkg_sandbox.Wrap(execCtx, runtime, []string{data.scriptPath}, workDir, nil)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Denied, "sandbox.Wrap", "", err)
	}

	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "cmd.Start", "", err)
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
					return "", utilsmsg.New(utilsmsg.Upstream, "script", "error: %s", strings.TrimSpace(stderr.String()))
				}
				if execCtx.Err() == context.DeadlineExceeded {
					return "", utilsmsg.New(utilsmsg.Timeout, "script execution", "timeout: %s", timeout)
				}
				return "", utilsmsg.Wrap(utilsmsg.Internal, "exec", "", err)
			}
			return strings.TrimSpace(stdout.String()), nil

		case <-ticker.C:
			slog.Debug("running",
				slog.String("name", key),
				slog.String("elapsed", fmt.Sprintf("%ds/%ds", int(time.Since(start).Seconds()), int(timeout.Seconds()))))
		}
	}
}

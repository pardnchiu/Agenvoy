package script

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	go_utils_sandbox "github.com/pardnchiu/go-utils/sandbox"
)

func Run(caller, scriptPath string) string {
	var binary string
	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".py":
		binary = "python3"
	default:
		binary = "sh"
	}

	workDir := filepath.Dir(scriptPath)
	cmd, err := go_utils_sandbox.Wrap(context.Background(), binary, []string{scriptPath}, workDir, nil)
	if err != nil {
		slog.Error(caller,
			slog.String("script", filepath.Base(scriptPath)),
			slog.String("error", err.Error()))
		return fmt.Sprintf("error: %s", err.Error())
	}

	cmd.Env = append(os.Environ(),
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/opt/homebrew/sbin",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		slog.Error(caller,
			slog.String("script", filepath.Base(scriptPath)),
			slog.String("error", err.Error()),
			slog.String("output", output))
		if output != "" {
			return fmt.Sprintf("error: %s\n%s", err.Error(), output)
		}
		return fmt.Sprintf("error: %s", err.Error())
	}
	return strings.TrimSpace(string(out))
}

func Remove(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("os.Remove",
			slog.String("script", path),
			slog.String("error", err.Error()))
	}
}

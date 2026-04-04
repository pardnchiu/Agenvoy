package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
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

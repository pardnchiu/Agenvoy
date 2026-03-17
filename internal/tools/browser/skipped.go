package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func isSkipped(href string) bool {
	for _, folder := range []string{"4xx", "5xx"} {
		_, path, err := skippedPath(folder, href)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func skippedPath(folder, href string) (string, string, error) {
	cached := filepath.Join(filesystem.ToolsDir, "browser", folder)
	// configDir, err := utils.GetConfigDir("tools", "browser", folder)
	// if err != nil {
	// 	return "", "", err
	// }
	hash := sha256.Sum256([]byte(href))
	name := hex.EncodeToString(hash[:])
	return cached, filepath.Join(cached, name), nil
}

func addToSkippedMap(href string, status int) {
	folder := "4xx"
	if status >= 500 {
		folder = "5xx"
	}

	dir, path, err := skippedPath(folder, href)
	if err != nil {
		slog.Warn("skippedPath",
			slog.String("error", err.Error()))
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("os.MkdirAll",
			slog.String("error", err.Error()))
		return
	}

	if err = filesystem.WriteFile(path, "1", 0644); err != nil {
		slog.Warn("utils.WriteFile",
			slog.String("error", err.Error()))
	}
}

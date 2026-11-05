package tools

import (
	"log/slog"
	"path/filepath"
	"sync"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var legacyAPIToolOnce sync.Once

// ! will deprecate in v2.*.*, no longer usage, just call with link
func warnLegacyAPITools() {
	legacyAPIToolOnce.Do(func() {
		for _, dir := range []string{
			filepath.Join(filesystem.ToolsDir, "api"),
			filepath.Join(filesystem.ToolsDir, ".extension", "api"),
			filepath.Join(filesystem.WorkAgenvoyDir, "tools", "api"),
		} {
			files, err := go_pkg_filesystem_reader.ListFiles(dir)
			if err != nil {
				continue
			}
			for _, file := range files {
				if file.IsDir || filepath.Ext(file.Name) != ".json" {
					continue
				}
				slog.Warn("[Invalid] api tool: no longer loaded; rewrite as a script tool or call the endpoint with http_request",
					slog.String("path", file.Path))
			}
		}
	})
}

package record

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

const (
	toolCallMaxAge = 7 * 24 * time.Hour
)

func CleanToolCalls() {
	if !go_pkg_filesystem_reader.IsDir(filesystem.SessionsDir) {
		return
	}

	sessions, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		slog.Warn("record CleanToolCalls ReadDir sessions",
			slog.String("error", err.Error()))
		return
	}

	cutoff := time.Now().Add(-toolCallMaxAge)
	cutoffDate := cutoff.Format("2006-01-02")

	for _, sess := range sessions {
		if !sess.IsDir() || sess.Name() == ".Trash" {
			continue
		}
		tcDir := filepath.Join(filesystem.SessionsDir, sess.Name(), "tool_calls")
		if !go_pkg_filesystem_reader.IsDir(tcDir) {
			continue
		}

		dates, err := os.ReadDir(tcDir)
		if err != nil {
			continue
		}
		for _, d := range dates {
			if !d.IsDir() {
				continue
			}
			if d.Name() >= cutoffDate {
				continue
			}
			if err := os.RemoveAll(filepath.Join(tcDir, d.Name())); err != nil {
				slog.Warn("record CleanToolCalls RemoveAll",
					slog.String("path", filepath.Join(tcDir, d.Name())),
					slog.String("error", err.Error()))
			}
		}
	}
}

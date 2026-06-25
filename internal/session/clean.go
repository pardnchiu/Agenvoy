package session

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func Clean() {
	entries, err := os.ReadDir(filesystem.SessionsDir)
	if err != nil {
		slog.Warn("os ReadDir",
			slog.String("dir", filesystem.SessionsDir),
			slog.String("error", err.Error()))
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionDir := filesystem.SessionDir(entry.Name())

		if strings.HasPrefix(entry.Name(), "temp-") {
			if now.Sub(latestModTime(sessionDir)) > time.Hour {
				if err := os.RemoveAll(sessionDir); err != nil {
					slog.Warn("os RemoveAll",
						slog.String("dir", entry.Name()),
						slog.String("error", err.Error()))
				}
			}
			continue
		}

		cleanTaskHistory(sessionDir, now)
	}
}


func cleanTaskHistory(sessionDir string, now time.Time) {
	histDir := filepath.Join(sessionDir, "history")
	files, err := os.ReadDir(histDir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > 3*24*time.Hour {
			os.Remove(filepath.Join(histDir, f.Name()))
		}
	}
}

func latestModTime(dir string) time.Time {
	var latest time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			slog.Warn("DirEntry Info",
				slog.String("entry", d.Name()),
				slog.String("error", err.Error()))
			return nil
		}
		if t := info.ModTime(); t.After(latest) {
			latest = t
		}
		return nil
	})
	return latest
}

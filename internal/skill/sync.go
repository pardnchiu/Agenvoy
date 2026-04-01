package skill

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func SyncSkills(_ context.Context, fsys fs.FS) {
	if err := os.RemoveAll(filesystem.SystemSkillsDir); err != nil {
		slog.Error("os.RemoveAll",
			slog.String("path", filesystem.SystemSkillsDir),
			slog.String("error", err.Error()))
		return
	}

	if err := os.MkdirAll(filesystem.SystemSkillsDir, 0755); err != nil {
		slog.Error("os.MkdirAll",
			slog.String("path", filesystem.SystemSkillsDir),
			slog.String("error", err.Error()))
		return
	}

	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		slog.Error("fs.ReadDir",
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		srcDir := "skills/" + entry.Name()
		destDir := filepath.Join(filesystem.SystemSkillsDir, entry.Name())

		if err := copyFromFS(fsys, srcDir, destDir); err != nil {
			slog.Warn("copyFromFS",
				slog.String("skill", entry.Name()),
				slog.String("error", err.Error()))
		}
	}
}

func copyFromFS(fsys fs.FS, srcDir, destDir string) error {
	return fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel := strings.TrimPrefix(path, srcDir)
		rel = strings.TrimPrefix(rel, "/")
		destPath := filepath.Join(destDir, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("fs.ReadFile %s: %w", path, err)
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

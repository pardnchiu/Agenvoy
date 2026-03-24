package filesystem

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func WalkFiles(dirs ...string) ([]string, error) {
	if len(dirs) == 0 || len(dirs) > 2 {
		return nil, fmt.Errorf("invalid dir: %d", len(dirs))
	}
	workDir := dirs[0]
	subDir := workDir
	if len(dirs) == 2 {
		subDir = dirs[1]
	}

	absPath, err := AbsPath(workDir, subDir, false)
	if err != nil {
		return nil, fmt.Errorf("AbsPath: %w", err)
	}

	var files []string
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("filepath.WalkDir",
				slog.String("error", err.Error()))
			return nil
		}

		if isExclude(workDir, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(absPath, path)
		if err != nil {
			slog.Warn("filepath.Rel",
				slog.String("error", err.Error()))
			return nil
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf(" filepath.WalkDir: %w", err)
	}
	return files, nil
}

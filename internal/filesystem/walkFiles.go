package filesystem

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
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

	absPath, err := go_utils_filesystem.AbsPath(workDir, subDir, go_utils_filesystem.AbsPathOption{HomeOnly: true})
	if err != nil {
		return nil, fmt.Errorf("go_utils_filesystem.AbsPath: %w", err)
	}

	var files []string
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("filepath.WalkDir",
				slog.String("error", err.Error()))
			return nil
		}

		if go_utils_filesystem.IsExcluded(workDir, path) {
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

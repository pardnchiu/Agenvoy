package filesystem

import (
	"fmt"
	"os"
	"path/filepath"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

func ListDir(dirs ...string) ([]os.DirEntry, error) {
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

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("os.ReadDir: %w", err)
	}

	var files []os.DirEntry
	for _, entry := range entries {
		newPath := filepath.Join(absPath, entry.Name())
		if go_utils_filesystem.IsExcluded(workDir, newPath) {
			continue
		}
		files = append(files, entry)
	}
	return files, nil
}

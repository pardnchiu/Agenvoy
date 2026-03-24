package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

func ListDir(dirs ...string) ([]string, error) {
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

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("os.ReadDir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		newPath := filepath.Join(absPath, entry.Name())
		if isExclude(workDir, newPath) {
			continue
		}

		if entry.IsDir() {
			files = append(files, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

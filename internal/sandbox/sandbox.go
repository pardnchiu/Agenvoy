package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func vaildateDir(path string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("os.UserHomeDir: %w", err)
	}

	abs, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("filepath.EvalSymlinks: %w", err)
	}

	if !strings.HasPrefix(abs, homeDir+"/") && abs != homeDir {
		return "", fmt.Errorf("just allow paths under home: %s", abs)
	}

	return homeDir, nil
}

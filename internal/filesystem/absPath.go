package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AbsPath(dir, path string) (string, error) {
	// * expand ~ to home directory
	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("os.UserHomeDir: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	}

	// * format the path to abs path
	if !filepath.IsAbs(path) {
		if dir != "" {
			path = filepath.Join(dir, path)
		} else {
			var err error
			path, err = filepath.Abs(path)
			if err != nil {
				return "", fmt.Errorf("filepath.Abs: %w", err)
			}
		}
	}

	realPath, err := realPath(path)
	if err != nil {
		return "", fmt.Errorf("realPath: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("os.UserHomeDir: %w", err)
	}

	homePrefix := homeDir + string(filepath.Separator)
	if !strings.HasPrefix(realPath, homePrefix) {
		return "", fmt.Errorf("only allow path under user home: %s", path)
	}

	if isDenied(realPath) {
		return "", fmt.Errorf("access denied: %s", path)
	}

	return realPath, nil
}

// * prevent symlinks to the path not under home
func realPath(path string) (string, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(path))
		if parentErr != nil {
			return "", fmt.Errorf("filepath.EvalSymlinks: %w", parentErr)
		}
		realPath = filepath.Join(realParent, filepath.Base(path))
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("filepath.EvalSymlinks: %w", err)
	}
	return realPath, nil
}

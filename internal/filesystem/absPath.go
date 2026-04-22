package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func AbsPath(root, path string, needExclude bool) (string, error) {
	path = strings.TrimSpace(path)

	// * resolve starting anchor
	switch {
	case path == "":
		path = root
	case path == "~" || strings.HasPrefix(path, "~/"):
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("os.UserHomeDir: %w", err)
		}
		path = filepath.Join(homeDir, path[1:])
	case path == "." || strings.HasPrefix(path, "./"):
		path = filepath.Join(root, strings.TrimPrefix(path, "./"))
	case !filepath.IsAbs(path):
		path = filepath.Join(root, path)
	}

	// * fallback when workDir is also empty
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("filepath.Abs: %w", err)
		}
		path = abs
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
	if realPath != homeDir && !strings.HasPrefix(realPath, homePrefix) {
		return "", fmt.Errorf("only allow path under user home: %s", path)
	}

	if isDenied(realPath) {
		return "", fmt.Errorf("access denied: %s", path)
	}

	if needExclude && isExclude(root, realPath) {
		return "", fmt.Errorf("path is excluded: %s", path)
	}

	return realPath, nil
}

// * prevent symlinks to the path not under home
func realPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("filepath.EvalSymlinks: %w", err)
	}

	// * walk up until path is found, then reconstruct
	suffix := []string{filepath.Base(path)}
	dir := filepath.Dir(path)
	for {
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("filepath.EvalSymlinks: no existing path for %s", path)
		}
		realAncestor, parentErr := filepath.EvalSymlinks(dir)
		if parentErr == nil {
			parts := append([]string{realAncestor}, suffix...)
			return filepath.Join(parts...), nil
		}
		if !errors.Is(parentErr, os.ErrNotExist) {
			return "", fmt.Errorf("filepath.EvalSymlinks: %w", parentErr)
		}
		suffix = append([]string{filepath.Base(dir)}, suffix...)
		dir = filepath.Dir(dir)
	}
}

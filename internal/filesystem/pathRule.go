package filesystem

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

func IsDeniedPath(absPath string) bool {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if absPath == "" || absPath == "." {
		return false
	}
	sep := string(filepath.Separator)
	return slices.ContainsFunc(DeniedPath, func(one string) bool {
		return absPath == one || strings.HasPrefix(absPath, one+sep)
	})
}

func IsSensitivePath(absPath string) bool {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if absPath == "" || absPath == "." {
		return false
	}
	dic := SensitivePath

	for segment := range strings.SplitSeq(absPath, string(filepath.Separator)) {
		if slices.Contains(dic.Dirs, segment) {
			return true
		}
	}

	base := filepath.Base(absPath)
	if slices.Contains(dic.Files, base) {
		return true
	}
	if slices.ContainsFunc(dic.Prefixes, func(p string) bool { return strings.HasPrefix(base, p) }) {
		return true
	}
	return slices.Contains(dic.Extensions, strings.ToLower(filepath.Ext(base)))
}

func IsWithin(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func RealHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if real, err := go_pkg_filesystem.RealPath(home); err == nil {
		return real, nil
	}
	return home, nil
}

func WorkDirAllowsAll(workDir string) bool {
	if strings.TrimSpace(workDir) == "" {
		return false
	}
	home, err := RealHome()
	if err != nil {
		return false
	}
	dir, err := go_pkg_filesystem.RealPath(workDir)
	if err != nil {
		return false
	}
	return IsWithin(home, dir) && dir != filepath.Clean(home) && !IsDeniedPath(dir) && !IsSensitivePath(dir)
}

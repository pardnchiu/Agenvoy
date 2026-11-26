package filesystem

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
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

func IsReadOnlyPath(absPath string) bool {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if absPath == "" || absPath == "." {
		return false
	}
	sep := string(filepath.Separator)
	deepest := func(list []string) int {
		depth := -1
		for _, one := range list {
			if absPath == one || strings.HasPrefix(absPath, one+sep) {
				depth = max(depth, len(one))
			}
		}
		return depth
	}
	readOnly := deepest(ReadOnlyPath)
	return readOnly >= 0 && readOnly > deepest(WritablePath)
}

func IsReadablePath(absPath string) bool {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if !filepath.IsAbs(absPath) {
		return false
	}
	if real, err := go_pkg_filesystem.RealPath(absPath); err == nil {
		absPath = real
	}
	if IsDeniedPath(absPath) || IsSensitivePath(absPath) {
		return false
	}
	return go_pkg_filesystem.IsReadable(absPath) || slices.ContainsFunc(ReadablePath, func(root string) bool {
		return IsWithin(root, absPath)
	})
}

func IsOffLimitsReadArg(arg string) bool {
	if IsSensitivePath(arg) {
		return true
	}
	if _, value, ok := strings.Cut(arg, "="); ok && strings.HasPrefix(arg, "-") {
		arg = value
	}
	if !strings.HasPrefix(arg, "/") {
		return false
	}
	return IsUnreadableOutsideHome(arg)
}

func IsUnreadableOutsideHome(absPath string) bool {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if !filepath.IsAbs(absPath) {
		return false
	}
	if _, err := os.Lstat(absPath); err != nil {
		return false
	}
	home, err := RealHome()
	if err != nil {
		return true
	}
	real, err := go_pkg_filesystem.RealPath(absPath)
	if err != nil {
		return true
	}
	return !IsWithin(home, real) && !IsReadablePath(real)
}

func IsSensitivePath(absPath string) bool {
	return go_pkg_filesystem.IsSensitive(absPath, SensitivePath)
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
	return IsWithin(home, dir) && dir != filepath.Clean(home) && !IsDeniedPath(dir) && !IsReadOnlyPath(dir) && !IsSensitivePath(dir)
}

func CheckDirName(field, name string) error {
	switch {
	case strings.TrimSpace(name) == "":
		return utilsmsg.New(utilsmsg.Invalid, field, "is required")
	case strings.ContainsAny(name, `/\`):
		return utilsmsg.New(utilsmsg.Invalid, field, "%q cannot contain a path separator", name)
	case strings.HasPrefix(name, "."):
		return utilsmsg.New(utilsmsg.Invalid, field, "%q cannot start with a dot", name)
	}
	return nil
}

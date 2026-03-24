package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteFile(path, content string, permission os.FileMode) error {
	if isDenied(path) {
		return fmt.Errorf("directory is denied: %s", path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}

	// * ensure atomic write:
	// * pre-save data as temp
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), permission); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}

	// * rename temp to target
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("os.Rename: %w", err)
	}
	return nil
}

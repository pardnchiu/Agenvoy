package file

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func write(e *toolTypes.Executor, path, content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("refused to write empty content to file (%s)", path)
	}

	fullPath := getFullPath(e, path)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory (%s): %w", path, err)
	}

	if err := filesystem.WriteFile(fullPath, content, 0644); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", path, err)
	}

	return fmt.Sprintf("Successfully wrote file: %s", path), nil
}

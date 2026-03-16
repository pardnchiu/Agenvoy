package file

import (
	"fmt"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func patch(e *toolTypes.Executor, path, oldString, newString string) (string, error) {
	fullPath, err := getFullPath(e, path)
	if err != nil {
		return "", err
	}

	if isExclude(e, fullPath) {
		return "", fmt.Errorf("path is excluded: %s", path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file (%s): %w", path, err)
	}

	content := string(data)
	if !strings.Contains(content, oldString) {
		return "", fmt.Errorf("old_string not found in file: %s", path)
	}

	newContent := strings.Replace(content, oldString, newString, 1)
	if err := filesystem.WriteFile(e.WorkPath, fullPath, newContent, 0644); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", err)
	}

	return fmt.Sprintf("Successfully patched: %s", path), nil
}

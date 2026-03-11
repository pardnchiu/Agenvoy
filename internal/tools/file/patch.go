package file

import (
	"fmt"
	"os"
	"strings"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func patch(e *toolTypes.Executor, path, oldString, newString string) (string, error) {
	fullPath := getFullPath(e, path)

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
	if err := utils.WriteFile(fullPath, newContent, 0644); err != nil {
		return "", fmt.Errorf("utils.WriteFile: %w", err)
	}

	return fmt.Sprintf("Successfully patched: %s", path), nil
}

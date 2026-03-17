package file

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func glob(e *toolTypes.Executor, pattern string) (string, error) {
	pattern = filepath.ToSlash(pattern)
	patterns := strings.Split(pattern, "/")

	files, err := filesystem.WalkFiles(e.WorkPath)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, file := range files {
		parts := strings.Split(file, "/")
		if matchFiles(patterns, parts) {
			sb.WriteString(file + "\n")
		}
	}

	if sb.Len() == 0 {
		return fmt.Sprintf("No files found in %s", pattern), nil
	}
	return sb.String(), nil
}

func matchFiles(patterns, parts []string) bool {
	if len(patterns) == 0 {
		return len(parts) == 0
	}

	pattern := patterns[0]
	if pattern == "**" {
		rest := patterns[1:]
		for i := 0; i <= len(parts); i++ {
			if matchFiles(rest, parts[i:]) {
				return true
			}
		}
		return false
	}

	if len(parts) == 0 {
		return false
	}

	match, err := filepath.Match(pattern, parts[0])
	if err != nil || !match {
		return false
	}
	return matchFiles(patterns[1:], parts[1:])
}

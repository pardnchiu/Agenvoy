package file

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func search(e *toolTypes.Executor, pattern, filePattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex pattern (%s): %w", pattern, err)
	}

	var result strings.Builder

	err = filepath.Walk(e.WorkPath, func(path string, d os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("failed to access path, just skipping",
				slog.String("error", err.Error()))
			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		exts := map[string]bool{
			".exe":   true,
			".bin":   true,
			".so":    true,
			".dylib": true,
			".dll":   true,
			".o":     true,
			".a":     true,
		}
		if exts[ext] {
			return nil
		}

		if filePattern != "" {
			matched, err := filepath.Match(filePattern, filepath.Base(path))
			if err != nil {
				slog.Warn("failed to match pattern",
					slog.String("error", err.Error()))
			}
			if !matched {
				return nil
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("failed to read file during search",
				slog.String("error", err.Error()))
			return nil
		}

		lines := strings.Split(string(data), "\n")
		relPath, err := filepath.Rel(e.WorkPath, path)
		if err != nil {
			slog.Warn("failed to get relative path",
				slog.String("error", err.Error()))
			return nil
		}

		for i, line := range lines {
			if re.MatchString(line) {
				result.WriteString(fmt.Sprintf("%s:%d: %s\n", relPath, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory (%s): %w", pattern, err)
	}

	if result.Len() == 0 {
		return fmt.Sprintf("No fils found: %s", pattern), nil
	}
	return result.String(), nil
}

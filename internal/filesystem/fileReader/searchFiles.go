package fileReader

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var binaryExts = map[string]bool{
	".exe":   true,
	".bin":   true,
	".so":    true,
	".dylib": true,
	".dll":   true,
	".o":     true,
	".a":     true,
}

func SearchFiles(absPath, pattern string, filePatterns []string) (string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex pattern (%s): %w", pattern, err)
	}

	var result strings.Builder
	err = filepath.Walk(absPath, func(path string, d os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("failed to access path, just skipping",
				slog.String("error", err.Error()))
			return nil
		}

		basePath := filepath.Base(path)
		if strings.HasPrefix(basePath, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if d.Size() > maxReadSize {
			return nil
		}

		if binaryExts[filepath.Ext(path)] {
			return nil
		}

		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			slog.Warn("failed to get relative path",
				slog.String("error", err.Error()))
			return nil
		}

		if len(filePatterns) > 0 {
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if !filesystem.IsMatch(filePatterns, parts) {
				return nil
			}
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if bytes.IndexByte(data[:min(len(data), 512)], 0) >= 0 {
			return nil
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if regex.MatchString(line) {
				result.WriteString(fmt.Sprintf("%s:%d: %s\n", relPath, lineNum, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory (%s): %w", pattern, err)
	}

	if result.Len() == 0 {
		return fmt.Sprintf("no files found: %s", pattern), nil
	}
	return result.String(), nil
}

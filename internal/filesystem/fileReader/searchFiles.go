package fileReader

import (
	"bufio"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func SearchFiles(absPath, pattern string, filePatterns []string) (string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("regexp.Compile (%s): %w", pattern, err)
	}

	matches, err := go_pkg_filesystem_reader.SearchFiles(absPath, pattern, filePatterns, 0,
		go_pkg_filesystem_reader.ListOption{
			SkipExcluded:    true,
			IgnoreWalkError: true,
		})
	if err != nil {
		return "", fmt.Errorf("go_pkg_filesystem_reader.SearchFiles: %w", err)
	}

	var result strings.Builder
	for _, path := range matches {
		text, err := go_pkg_filesystem.ReadText(path)
		if err != nil {
			slog.Warn("go_pkg_filesystem.ReadText",
				slog.String("path", path),
				slog.String("error", err.Error()))
			continue
		}
		relPath, err := filepath.Rel(absPath, path)
		if err != nil {
			relPath = path
		}
		scanner := bufio.NewScanner(strings.NewReader(text))
		scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if regex.MatchString(line) {
				fmt.Fprintf(&result, "%s:%d: %s\n", relPath, lineNum, strings.TrimSpace(line))
			}
		}
	}

	if result.Len() == 0 {
		return fmt.Sprintf("no files found: %s", pattern), nil
	}
	return result.String(), nil
}

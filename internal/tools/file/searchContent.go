package file

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
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

func registSearchContent() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "search_content",
		Description: "Search file contents for a pattern. Returns matching lines with file path and line number.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Text or regex pattern to search for",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "Optional glob pattern to filter files (e.g. '**/*.go')",
				},
			},
			"required": []string{"pattern"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				FilePattern string `json:"file_pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			return search(e, params.Pattern, params.FilePattern)
		},
	})
}

func search(e *toolTypes.Executor, pattern, filePattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex pattern (%s): %w", pattern, err)
	}

	var filePatternParts []string
	if filePattern != "" {
		filePatternParts = strings.Split(filepath.ToSlash(filePattern), "/")
	}

	var result strings.Builder

	err = filepath.Walk(e.WorkDir, func(path string, d os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("failed to access path, just skipping", slog.String("error", err.Error()))
			return nil
		}

		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") {
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

		relPath, err := filepath.Rel(e.WorkDir, path)
		if err != nil {
			slog.Warn("failed to get relative path", slog.String("error", err.Error()))
			return nil
		}

		if len(filePatternParts) > 0 {
			parts := strings.Split(filepath.ToSlash(relPath), "/")
			if !filesystem.IsMatch(filePatternParts, parts) {
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
			if re.MatchString(line) {
				result.WriteString(fmt.Sprintf("%s:%d: %s\n", relPath, lineNum, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory (%s): %w", pattern, err)
	}

	if result.Len() == 0 {
		return fmt.Sprintf("No files found: %s", pattern), nil
	}
	return result.String(), nil
}

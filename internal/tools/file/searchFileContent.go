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

func registSearchFileContent() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "search_file_content",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Search file contents by RE2 regex.
Locate code or text when the matching string is known but the file is not.
Scope with file_pattern glob (e.g. '**/*.go', 'configs/**').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "RE2 regex matched per line (e.g. 'func\\s+\\w+Handler', 'TODO:', 'api_key').",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "Glob to narrow files (e.g. '**/*.go', 'configs/**/*.json').",
				},
			},
			"required": []string{
				"pattern",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Pattern     string `json:"pattern"`
				FilePattern string `json:"file_pattern"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			textPattern := strings.TrimSpace(params.Pattern)
			if textPattern == "" {
				return "", fmt.Errorf("text_pattern is required")
			}

			var filePatterns []string
			if params.FilePattern != "" {
				filePatterns = strings.Split(filepath.ToSlash(params.FilePattern), "/")
			}
			return searchContentHandler(e, textPattern, filePatterns)
		},
	})
}

func searchContentHandler(e *toolTypes.Executor, pattern string, filePatterns []string) (string, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex pattern (%s): %w", pattern, err)
	}

	baseDir := e.WorkDir
	if baseDir == "" {
		baseDir = filesystem.DownloadDir
	}

	var result strings.Builder
	err = filepath.Walk(baseDir, func(path string, d os.FileInfo, err error) error {
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

		relPath, err := filepath.Rel(baseDir, path)
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

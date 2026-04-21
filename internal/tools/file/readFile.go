package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	maxReadSize      = 1 << 20
	defaultReadLimit = 2048
)

func registReadFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "read_file",
		ReadOnly: true,
		Description: `
Read the contents of a text or .pdf file.
Inspect source code, config, notes, or extract text from a PDF.
Accepts absolute paths and '~' (e.g. '/abs/path/foo.go', '~/notes.md').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to read (e.g. '/abs/path/foo.go', '~/notes.md', 'relative/file.md').",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-based line number (or PDF page) to start from. Defaults to 1.",
					"default":     1,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Number of lines (or PDF pages) to read. Defaults to 2048.",
					"default":     defaultReadLimit,
				},
			},
			"required": []string{
				"path",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			absPath, err := filesystem.AbsPath(baseDir, params.Path, true)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}

			offset := max(params.Offset, 1)
			limit := max(params.Limit, 0)
			if limit == 0 {
				limit = defaultReadLimit
			}
			return readFileHandler(absPath, offset, limit)
		},
	})
}

func readFileHandler(path string, offset, limit int) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		return readPDFHandler(path, offset, limit)
	}
	if imageExts[ext] {
		return "", fmt.Errorf("image file detected, use `read_image` instead")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	fileContent := string(fileBytes)
	if strings.IndexByte(fileContent[:min(len(fileContent), 512)], 0) >= 0 {
		return "", fmt.Errorf("binary file: %s", path)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(fileContent))
	scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
	lineNum := 0
	written := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if written >= limit {
			break
		}
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
		written++
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("bufio.Scanner: %w", err)
	}
	if sb.Len() == 0 {
		if lineNum == 0 {
			return fmt.Sprintf("%s is empty", path), nil
		}
		return fmt.Sprintf("offset %d exceeds total lines %d in %s", offset, lineNum, path), nil
	}
	return sb.String(), nil
}

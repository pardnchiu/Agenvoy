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
		Name:        "read_file",
		Description: "Read the contents of a file. Use to inspect source code, config files, or any text file in the project. Supports .pdf (text extraction).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file (relative to project root or absolute)",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "Line number to start reading from (1-based). For .pdf files, interpreted as page number. Omit to read from the beginning.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Number of lines to read. Defaults to 2048. For .pdf files, interpreted as number of pages. Pass a larger value if the file requires more lines.",
				},
			},
			"required": []string{"path"},
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

			absPath, err := filesystem.AbsPath(e.WorkDir, params.Path, true)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}

			if strings.ToLower(filepath.Ext(absPath)) == ".pdf" {
				return readPDF(absPath, params.Offset, params.Limit)
			}

			content, _, err := readFile(e, params.Path)
			if err != nil {
				return "", err
			}

			if strings.IndexByte(content[:min(len(content), 512)], 0) >= 0 {
				return "", fmt.Errorf("binary file: %s", params.Path)
			}

			limit := params.Limit
			if limit == 0 {
				limit = defaultReadLimit
			}
			startLine := max(params.Offset, 1)

			var sb strings.Builder
			scanner := bufio.NewScanner(strings.NewReader(content))
			scanner.Buffer(make([]byte, maxReadSize), maxReadSize)
			lineNum := 0
			written := 0
			for scanner.Scan() {
				lineNum++
				if lineNum < startLine {
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
				return fmt.Sprintf("%s is empty", params.Path), nil
			}
			return sb.String(), nil
		},
	})
}

func readFile(e *toolTypes.Executor, path string) (string, string, error) {
	absPath, err := filesystem.AbsPath(e.WorkDir, path, true)
	if err != nil {
		return "", "", fmt.Errorf("filesystem.AbsPath: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", absPath, fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", absPath, fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", absPath, fmt.Errorf("os.ReadFile: %w", err)
	}
	return string(data), absPath, nil
}

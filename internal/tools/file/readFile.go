package file

import (
	"context"
	"encoding/json"
	"fmt"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/fileReader"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	maxReadSize      = 1 << 20
	defaultReadLimit = 2048
)

func registReadFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "read_file",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Read a text, PDF, CSV/TSV, or image file.
Inspect source, config, notes, tabular data, or screenshots.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to read (e.g. '/abs/path/foo.go', '~/notes.md', 'relative/file.md').",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-based line (or page for PDF, row for CSV). Defaults to 1.",
					"default":     1,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Lines (or pages for PDF, rows for CSV) to read. Defaults to 2048.",
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

			absPath, err := go_utils_filesystem.AbsPath(baseDir, params.Path, go_utils_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_utils_filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}

			offset := max(params.Offset, 1)
			limit := max(params.Limit, 0)
			if limit == 0 {
				limit = defaultReadLimit
			}
			return fileReader.ReadFile(absPath, offset, limit)
		},
	})
}

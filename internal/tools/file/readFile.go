package file

import (
	"context"
	"encoding/json"
	"fmt"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/file/denied"
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
		AlwaysAllow: true,
		Concurrent:  true,
		Description: "Read a text, PDF, DOCX, PPTX, CSV/TSV, or image file. Must be called before patch_file (skip if already read this session). Also call after patch_file/write_file to verify the edit landed correctly.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to read (e.g. '/abs/path/foo.go', '~/notes.md', 'relative/file.md').",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-based line (page for PDF, slide for PPTX, row for CSV). Defaults to 1.",
					"default":     1,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Lines (pages for PDF, slides for PPTX, rows for CSV) to read. Defaults to 2048.",
					"default":     defaultReadLimit,
				},
			},
			"required": []string{
				"path",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
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

			absPath, err := go_pkg_filesystem.AbsPath(baseDir, params.Path, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}

			if parent, ok := denied.Hit(e.SessionID, absPath); ok {
				return "", fmt.Errorf("permission denied: %s is under previously rejected %s; not retried", absPath, parent)
			}

			offset := max(params.Offset, 1)
			limit := max(params.Limit, 0)
			if limit == 0 {
				limit = defaultReadLimit
			}
			out, err := filesystem.ReadFile(ctx, absPath, offset, limit)
			if err != nil && denied.IsPermission(err) {
				denied.Register(e.SessionID, absPath)
				return "", fmt.Errorf("permission denied: %s (recorded; further reads under this path will be skipped)", absPath)
			}
			return out, err
		},
	})
}

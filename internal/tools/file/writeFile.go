package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/tools/file/denied"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteFile() {
	toolRegister.Regist(toolRegister.Def{
		Name: "write_file",
		Description: `
Write content to a file, overwriting if it exists.
Accepts absolute paths and '~' (e.g. '/abs/path/foo.go', '~/notes.md').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to write (e.g. '/abs/path/foo.go', '~/notes.md').",
					"default":     "",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write.",
				},
			},
			"required": []string{
				"content",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			path := strings.TrimSpace(params.Path)

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			absPath, err := go_pkg_filesystem.AbsPath(baseDir, path, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}

			content := params.Content
			if content == "" {
				return "", fmt.Errorf("content is required")
			}

			if parent, ok := denied.Hit(e.SessionID, absPath); ok {
				return "", fmt.Errorf("permission denied: %s is under previously rejected %s; not retried", absPath, parent)
			}

			info, err := os.Stat(absPath)
			isNew := os.IsNotExist(err)
			if err != nil && !isNew {
				if denied.IsPermission(err) {
					denied.Register(e.SessionID, absPath)
					return "", fmt.Errorf("permission denied: %s (recorded; further writes under this path will be skipped)", absPath)
				}
				return "", fmt.Errorf("os.Stat: %w", err)
			}
			if !isNew && info.Size() > maxReadSize {
				return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
			}

			if err := go_pkg_filesystem.WriteFile(absPath, content, 0644); err != nil {
				if denied.IsPermission(err) {
					denied.Register(e.SessionID, absPath)
					return "", fmt.Errorf("permission denied: %s (recorded; further writes under this path will be skipped)", absPath)
				}
				return "", fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
			}

			skill.AutoCommitByPath(ctx, absPath, isNew)

			if isNew {
				return fmt.Sprintf("successfully created: %s", absPath), nil
			}
			return fmt.Sprintf("successfully updated %s", absPath), nil
		},
	})
}

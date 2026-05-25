package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/tools/file/denied"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func registListFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_files",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `
List directory entries.
Inspect immediate children; recursive=true walks subtree files.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dir": map[string]any{
					"type":        "string",
					"description": "Directory to list (e.g. '.', '~/Desktop', '/abs/path'). Defaults to current working directory.",
					"default":     "",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "Walk subtree files. Default false.",
					"default":     false,
				},
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Dir       string `json:"dir"`
				Recursive bool   `json:"recursive"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			dir := strings.TrimSpace(params.Dir)
			absPath, err := go_pkg_filesystem.AbsPath(e.WorkDir, dir, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
			}

			if parent, ok := denied.Hit(e.SessionID, absPath); ok {
				return "", fmt.Errorf("permission denied: %s is under previously rejected %s; not retried", absPath, parent)
			}

			if f, openErr := os.Open(absPath); openErr != nil {
				if denied.IsPermission(openErr) {
					denied.Register(e.SessionID, absPath)
					return "", fmt.Errorf("permission denied: %s (recorded; further reads under this path will be skipped)", absPath)
				}
				return "", fmt.Errorf("os.Open: %w", openErr)
			} else {
				f.Close()
			}

			var files []go_pkg_filesystem_reader.File
			if params.Recursive {
				files, err = go_pkg_filesystem_reader.WalkFiles(absPath, go_pkg_filesystem_reader.ListOption{
					SkipExcluded:      true,
					IgnoreWalkError:   true,
					IncludeNonRegular: true,
				})
				if err != nil {
					return "", fmt.Errorf("go_pkg_filesystem_reader.WalkFiles: %w", err)
				}
			} else {
				files, err = go_pkg_filesystem_reader.ListAll(absPath, go_pkg_filesystem_reader.ListOption{SkipExcluded: true})
				if err != nil {
					return "", fmt.Errorf("go_pkg_filesystem_reader.ListAll: %w", err)
				}
			}

			data, err := json.Marshal(files)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(data), nil
		},
	})
}

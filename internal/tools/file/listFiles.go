package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "list_files",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
List directory entries.
Inspect immediate children; recursive=true walks subtree files.
Accepts absolute paths and '~' (e.g. '~/Desktop').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to list (e.g. '.', '~/Desktop', '/abs/path').",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "Walk subtree files. Default false.",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path      string `json:"path"`
				Recursive bool   `json:"recursive"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			absBase, err := filesystem.AbsPath(e.WorkDir, params.Path, false)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}

			results := []file{}

			if params.Recursive {
				walked, err := filesystem.WalkFiles(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
				}
				for _, rel := range walked {
					full := filepath.Join(absBase, rel)
					info, err := os.Stat(full)
					if err != nil {
						continue
					}
					results = append(results, file{
						Name:    info.Name(),
						Path:    full,
						IsDir:   info.IsDir(),
						Size:    info.Size(),
						ModTime: info.ModTime().Format("2006-01-02 15:04"),
					})
				}
			} else {
				entries, err := filesystem.ListDir(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.ListDir: %w", err)
				}
				for _, entry := range entries {
					full := filepath.Join(absBase, entry.Name())
					info, err := os.Stat(full)
					if err != nil {
						continue
					}
					results = append(results, file{
						Name:    info.Name(),
						Path:    full,
						IsDir:   info.IsDir(),
						Size:    info.Size(),
						ModTime: info.ModTime().Format("2006-01-02 15:04"),
					})
				}
			}

			data, err := json.Marshal(results)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(data), nil
		},
	})
}

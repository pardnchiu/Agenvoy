package file

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_files",
		Description: "列出指定路徑的檔案和目錄。用於探索專案結構。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "要列出的目錄路徑（相對於專案根目錄或絕對路徑）。使用 '.' 表示目前目錄。",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "如果為 true，則遞迴列出檔案。預設為 false。",
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
			if isDenied(params.Path) {
				return "", fmt.Errorf("access denied: %s", params.Path)
			}
			return list(e, params.Path, params.Recursive)
		},
	})
}

func list(e *toolTypes.Executor, path string, recursive bool) (string, error) {
	fullPath, err := getFullPath(e, path)
	if err != nil {
		return "", err
	}

	var files []string
	if recursive {
		files, err = walkFiles(e, fullPath)
	} else {
		files, err = listDir(e, fullPath)
	}
	if err != nil {
		return "", fmt.Errorf("list files — %w", err)
	}
	return strings.Join(files, "\n") + "\n", nil
}

func walkFiles(e *toolTypes.Executor, root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("failed to access path, just skipping",
				slog.String("error", err.Error()))
			return nil
		}

		if isExclude(e, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			slog.Warn("failed to get relative path, just skipping",
				slog.String("error", err.Error()))
			return nil
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files — %w", err)
	}
	return files, nil
}

func listDir(e *toolTypes.Executor, path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("list directory — %w", err)
	}

	var files []string
	for _, entry := range entries {
		newPath := filepath.Join(path, entry.Name())
		if isExclude(e, newPath) {
			continue
		}

		if entry.IsDir() {
			files = append(files, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

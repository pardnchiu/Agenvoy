package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registWriteFile() {
	toolRegister.Regist(toolRegister.Def{
		Name: "write_file",
		Description: `
Write content to a file, overwriting if it exists.
Create new files or fully rewrite existing ones. Set executable=true for scheduler scripts.
Accepts absolute paths and '~' (e.g. '/abs/path/foo.go', '~/notes.md').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File to write (e.g. '/abs/path/foo.go', '~/notes.md'). When executable is true, provide only the filename (e.g. 'notify.sh').",
					"default":     "",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Alias for path when executable is true (e.g. 'notify.sh').",
					"default":     "",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write.",
				},
				"executable": map[string]any{
					"type":        "boolean",
					"description": "If true, saves .sh or .py to the scheduler scripts directory with a timestamp suffix. Pass the returned filename to add_task or add_cron.",
					"default":     false,
				},
			},
			"required": []string{
				"content",
			},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path       string `json:"path"`
				Name       string `json:"name"`
				Content    string `json:"content"`
				Executable bool   `json:"executable"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			path := strings.TrimSpace(params.Path)
			if path == "" {
				path = strings.TrimSpace(params.Name)
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			absPath, err := go_pkg_filesystem.AbsPath(baseDir, path, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path or name is required")
			}

			content := params.Content
			if content == "" {
				return "", fmt.Errorf("content is required")
			}
			return write(ctx, absPath, content, params.Executable)
		},
	})
}

func write(ctx context.Context, path, content string, executable bool) (string, error) {
	if executable {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".sh" && ext != ".py" {
			return "", fmt.Errorf("executable scripts only support .sh or .py")
		}

		base := strings.TrimSuffix(filepath.Base(path), ext)
		uniqueName := fmt.Sprintf("%s_%d%s", base, time.Now().UTC().Unix(), ext)
		absPath := filepath.Join(filesystem.ScriptsDir, uniqueName)
		if err := go_pkg_filesystem.WriteFile(absPath, content, 0755); err != nil {
			return "", fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
		}
		return fmt.Sprintf(`script saved. pass "%s" as the script parameter to add_task or add_cron`, uniqueName), nil
	}

	info, err := os.Stat(path)
	isNew := os.IsNotExist(err)
	if err != nil && !isNew {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if !isNew && info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	if err := go_pkg_filesystem.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
	}

	filesystem.SkillCommit(ctx, path, isNew)

	if isNew {
		return fmt.Sprintf("successfully created: %s", path), nil
	}
	return fmt.Sprintf("successfully updated %s", path), nil
}

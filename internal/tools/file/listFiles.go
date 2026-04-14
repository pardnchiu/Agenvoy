package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListFiles() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_files",
		ReadOnly:    true,
		Description: "List files and directories at the specified path. Use for exploring project structure.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list. Absolute path preferred; relative paths resolve against the work directory shown in the system prompt. `~` expands to user home. Use '.' for the work directory.",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "If true, list files recursively. Defaults to false.",
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

			var sb strings.Builder

			if params.Recursive {
				files, err := filesystem.WalkFiles(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.WalkFiles: %w", err)
				}
				for _, f := range files {
					sb.WriteString("[file] ")
					sb.WriteString(f)
					sb.WriteByte('\n')
				}
			} else {
				entries, err := filesystem.ListDir(e.WorkDir, params.Path)
				if err != nil {
					return "", fmt.Errorf("filesystem.ListDir: %w", err)
				}
				for _, entry := range entries {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					if entry.IsDir() {
						sb.WriteString("[dir] ")
					} else {
						sb.WriteString("[file] ")
					}
					sb.WriteString(entry.Name())
					sb.WriteString(" / ")
					sb.WriteString(info.ModTime().Format("2006-01-02 15:04"))
					sb.WriteByte('\n')
				}
			}

			if sb.Len() == 0 {
				return fmt.Sprintf("%s no files found", params.Path), nil
			}
			return sb.String(), nil
		},
	})
}

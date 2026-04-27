package file

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem/fileReader"
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
			absPath, err := go_utils_filesystem.AbsPath(e.WorkDir, dir, go_utils_filesystem.AbsPathOption{HomeOnly: true})
			if err != nil {
				return "", fmt.Errorf("go_utils_filesystem.AbsPath: %w", err)
			}
			return fileReader.ListFiles(absPath, params.Recursive)
		},
	})
}

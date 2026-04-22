package file

import (
	"context"
	"encoding/json"
	"fmt"
	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/fileReader"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registReadImage() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "read_image",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Read a local image file as a base64 data URL for visual inspection.
Inspect JPEG, PNG, GIF, or WebP images referenced in the project.
Accepts absolute paths and '~' (e.g. '/abs/path/shot.png', '~/Pictures/img.jpg').`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Image file to read (e.g. '/abs/path/shot.png', '~/Pictures/img.jpg').",
				},
			},
			"required": []string{
				"path",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			baseDir := e.WorkDir
			if baseDir == "" {
				baseDir = filesystem.DownloadDir
			}

			absPath, err := filesystem.AbsPath(baseDir, params.Path, true)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}
			if absPath == "" {
				return "", fmt.Errorf("path is required")
			}
			return fileReader.ReadImage(absPath)
		},
	})
}

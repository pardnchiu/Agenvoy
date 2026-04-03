package file

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

func isImageExt(ext string) bool {
	return imageExts[strings.ToLower(ext)]
}

func registReadImage() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "read_image",
		ReadOnly: true,
		Description: `Read a local image file and return it as a base64 data URL so the model can visually inspect it.
Supports JPEG, PNG, GIF, WebP.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the image file (relative to project root or absolute)",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if params.Path == "" {
				return "", fmt.Errorf("path is required")
			}

			absPath, err := filesystem.AbsPath(e.WorkDir, params.Path, false)
			if err != nil {
				return "", fmt.Errorf("filesystem.AbsPath: %w", err)
			}

			info, err := os.Stat(absPath)
			if err != nil {
				return "", fmt.Errorf("os.Stat: %w", err)
			}
			const maxImageSize = 10 << 20
			if info.Size() > maxImageSize {
				return "", fmt.Errorf("image too large (%d bytes, max 10 MB)", info.Size())
			}

			f, err := os.Open(absPath)
			if err != nil {
				return "", fmt.Errorf("os.Open: %w", err)
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				return "", fmt.Errorf("image.Decode: %w", err)
			}

			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
				return "", fmt.Errorf("jpeg.Encode: %w", err)
			}

			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
		},
	})
}

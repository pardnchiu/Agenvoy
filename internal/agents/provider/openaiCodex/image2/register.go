package image2

import (
	"time"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

func Register() {

	toolRegister.Regist(toolRegister.Def{
		Name:        "generate_image",
		Timeout:     5 * time.Minute,
		Description: "Generate an image via gpt-image-2 on the codex@ subscription quota. Use when the user asks to create / draw / render an image. Confirm size and quality via ask_user first (no defaults; high ≈ 5× quota). Output path appears on the last line as `FILE: <path>`.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "Image description.",
				},
				"size": map[string]any{
					"type":        "string",
					"enum":        []string{"1024x1024", "1024x1792", "1792x1024"},
					"description": "Image dimensions. Confirm with the user via ask_user before calling.",
				},
				"quality": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "medium", "high"},
					"description": "Render quality. Confirm with the user via ask_user before calling (high ≈ 5x subscription quota per image).",
				},
				"reference_image_path": map[string]any{
					"type":        "string",
					"description": "Optional local path to an image for image-to-image. Supported: png, jpg, jpeg, webp.",
				},
			},
			"required": []string{"prompt", "size", "quality"},
		},
		Handler: handler,
	})
}

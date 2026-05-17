package image2

import (
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

func Register() {
	if !openaicodex.HasToken() {
		return
	}

	toolRegister.Regist(toolRegister.Def{
		Name: "generate_image",
		Description: `Generate an image via gpt-image-2 on the current codex@ subscription quota.
Required parameters: prompt, size, quality. If the user has not explicitly specified size or quality, you must first call ask_user to confirm both (do not guess defaults — quality=high costs ~5x more quota).
Output is written to ~/.config/agenvoy/download/ and the file path appears on the last line as "FILE: <path>".`,
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

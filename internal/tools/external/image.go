// * last modified: 26/11/18
package external

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	agentimage "github.com/pardnchiu/agenvoy/internal/agents/image"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func RegisterImage() {
	toolregister.Regist(toolregister.Def{
		Name:        "generate_image",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Timeout:     15 * time.Minute,
		Description: `[system-default] Generates an image from a text prompt and saves it.
Use for "draw a picture of", "generate an image", "make me a logo", and "edit this image" when a reference image is given.
Reading or describing an existing image → read_files; a real existing image such as a brand logo or a photo of a real place → download_file.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "What to draw, in English: subject, composition and style, e.g. 'a red fox in fresh snow, low angle, watercolor'. Some providers rewrite it and return the revision.",
				},
				"aspect_ratio": map[string]any{
					"type":        "string",
					"description": "Canvas shape as W:H, e.g. 1:1, 16:9, 4:3, 9:16.",
					"default":     "1:1",
				},
				"size": map[string]any{
					"type":        "string",
					"description": "Resolution tier: 1k, 2k or 4k (roughly 1024, 2048, 4096 px per edge); higher is slower.",
					"enum":        []string{"1k", "2k", "4k"},
					"default":     "1k",
				},
				"quality": map[string]any{
					"type":        "string",
					"description": "Render effort: low, medium, high; higher is slower. Ignored by gemini.",
					"enum":        []string{"low", "medium", "high"},
					"default":     "medium",
				},
				"reference": map[string]any{
					"type":        "string",
					"description": "Absolute path to a png/jpg/webp to edit or imitate; blank generates from the prompt alone.",
					"default":     "",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "Save path: blank auto-names image-<timestamp> in the output directory; absolute is used as-is; relative joins under the working directory; the extension always follows the returned format.",
					"default":     "",
				},
			},
			"required": []string{"prompt"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Prompt      string `json:"prompt"`
				AspectRatio string `json:"aspect_ratio"`
				Size        string `json:"size"`
				Quality     string `json:"quality"`
				Reference   string `json:"reference"`
				OutputFile  string `json:"output_file"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			if strings.TrimSpace(params.AspectRatio) == "" {
				params.AspectRatio = "1:1"
			}
			if strings.TrimSpace(params.Size) == "" {
				params.Size = "1k"
			}
			if strings.TrimSpace(params.Quality) == "" {
				params.Quality = "medium"
			}

			workDir := ""
			if e != nil {
				workDir = e.WorkDir
			}
			return agentimage.Generate(ctx, agentimage.Request{
				Prompt:      params.Prompt,
				AspectRatio: params.AspectRatio,
				Size:        params.Size,
				Quality:     params.Quality,
				Reference:   params.Reference,
				OutputFile:  params.OutputFile,
				WorkDir:     workDir,
			})
		},
	})
}

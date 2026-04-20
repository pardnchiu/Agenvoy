package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "analyze_youtube",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Analyze YouTube video content using Gemini for speech-to-text (STT), returning a complete transcript with timestamps.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "YouTube video URL (supports watch?v=, shorts/, youtu.be/ formats)",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "(Optional) Additional analysis instructions appended after the default full-transcript-with-timestamps prompt",
					"default":     "",
				},
			},
			"required": []string{
				"url",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				URL    string `json:"url"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			url := strings.TrimSpace(params.URL)
			if url == "" {
				return "", fmt.Errorf("url is required")
			}

			prompt := strings.TrimSpace(params.Prompt)
			return Fetch(ctx, url, prompt)
		},
	})
}

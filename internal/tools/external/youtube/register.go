package youtube

import (
	"context"
	"encoding/json"
	"fmt"

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
					"description": "Additional analysis instructions appended after the default full-transcript-with-timestamps prompt",
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

			if params.URL == "" {
				return "", fmt.Errorf("url is required")
			}
			return Fetch(ctx, params.URL, params.Prompt)
		},
	})
}

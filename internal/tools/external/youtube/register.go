package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:       "fetch_youtube_transcript",
		ReadOnly:   true,
		Concurrent: true,
		Description: `
Transcribe YouTube video with timestamps.
Video → text for analysis, summarization, quote extraction.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "YouTube URL (watch / shorts / youtu.be).",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Extra instructions appended to the default transcript prompt.",
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
			return handler(ctx, url, prompt)
		},
	})
}

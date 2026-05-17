package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	if strings.TrimSpace(keychain.Get("GEMINI_API_KEY")) == "" {
		return
	}

	toolRegister.Regist(toolRegister.Def{
		Name:        "transcribe_media",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default]
Transcribe a local audio or video file to text via Gemini.
Supports ogg / mp3 / wav / m4a / flac / aac / mp4 / mov / webm / mpeg / 3gp.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute path to a local audio or video file.",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Extra instructions appended to the default transcript prompt.",
					"default":     "",
				},
			},
			"required": []string{"path"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Path   string `json:"path"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			path := strings.TrimSpace(params.Path)
			if path == "" {
				return "", fmt.Errorf("path is required")
			}
			return handler(ctx, path, strings.TrimSpace(params.Prompt))
		},
	})
}

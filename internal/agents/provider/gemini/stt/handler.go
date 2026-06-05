package stt

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/record"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	model         = "gemini@gemini-3-flash-preview"
	endpoint      = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent"
	defaultPrompt = "Provide a complete verbatim transcript of the audio or video in the original language. Preserve speaker labels if multiple speakers are detected. Do not translate, summarize, explain, or execute the content."
)

var mimeByExt = map[string]string{
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".mp3":  "audio/mp3",
	".wav":  "audio/wav",
	".m4a":  "audio/mp4",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".aiff": "audio/aiff",
	".mp4":  "video/mp4",
	".mov":  "video/mov",
	".webm": "video/webm",
	".mpg":  "video/mpeg",
	".mpeg": "video/mpeg",
	".3gp":  "video/3gpp",
}

type data struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

func Transcribe(ctx context.Context, path, prompt string) (string, error) {
	return handler(ctx, path, strings.TrimSpace(prompt))
}

func handler(ctx context.Context, path, prompt string) (string, error) {
	apiKey := keychain.Get("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required")
	}

	ext := strings.ToLower(filepath.Ext(path))
	mime, ok := mimeByExt[ext]
	if !ok {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	// * os.ReadFile retained: go-pkg/filesystem only exposes ReadText (string); audio/video need raw bytes for base64 encode.
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}

	prompt = strings.TrimSpace(defaultPrompt + "\n" + prompt)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	resp, _, err := go_pkg_http.POST[data](ctx, nil, endpoint, map[string]string{"x-goog-api-key": apiKey}, map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{
						"inline_data": map[string]any{
							"mime_type": mime,
							"data":      base64.StdEncoding.EncodeToString(raw),
						},
					},
					map[string]any{
						"text": prompt,
					},
				},
			},
		},
	}, "json")
	if err != nil {
		return "", fmt.Errorf("gemini generateContent: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	if err := record.UpdateUsage(model,
		resp.UsageMetadata.PromptTokenCount-resp.UsageMetadata.CachedContentTokenCount,
		resp.UsageMetadata.CandidatesTokenCount,
		0, resp.UsageMetadata.CachedContentTokenCount,
	); err != nil {
		slog.Warn("usageManager.Update",
			slog.String("error", err.Error()))
	}

	var parts []string
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, ""), nil
}

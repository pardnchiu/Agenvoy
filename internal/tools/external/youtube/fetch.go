package youtube

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/go-utils/filesystem/keychain"
	go_utils_http "github.com/pardnchiu/go-utils/http"
)

const (
	model         = "gemini@gemini-3-flash-preview"
	path          = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent"
	defaultPrompt = "Please provide a complete verbatim transcript, including timestamps [MM:SS] and speaker identification. Do not summarize."
)

var regexVideoId = regexp.MustCompile(`(?:youtube\.com/(?:watch\?v=|shorts/|embed/)|youtu\.be/)([a-zA-Z0-9_-]{11})`)

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

func handler(ctx context.Context, videoURL, prompt string) (string, error) {
	apiKey := keychain.Get("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required")
	}

	if !regexVideoId.MatchString(videoURL) {
		return "", fmt.Errorf("failed to get videoId from url")
	}

	prompt = strings.TrimSpace(defaultPrompt + "\n" + prompt)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	resp, _, err := go_utils_http.POST[data](ctx, nil, path+"?key="+apiKey, nil, map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{
						"file_data": map[string]any{
							"file_uri": videoURL,
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
		return "", fmt.Errorf("failed to fetch gemini: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	if err := filesystem.UpdateUsage(model,
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

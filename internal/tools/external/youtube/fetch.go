package youtube

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/go-utils/filesystem/keychain"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	geminiAPI     = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent"
	defaultPrompt = "請提供完整逐字稿，包含時間戳記 [MM:SS] 與講者識別。不要摘要。"
)

var ytRegexp = regexp.MustCompile(`(?:youtube\.com/(?:watch\?v=|shorts/|embed/)|youtu\.be/)([a-zA-Z0-9_-]{11})`)

type geminiResponse struct {
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

func Fetch(ctx context.Context, videoURL, prompt string) (string, error) {
	apiKey := keychain.Get("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not configured")
	}

	if !ytRegexp.MatchString(videoURL) {
		return "", fmt.Errorf("ytRegexp.MatchString: %s", videoURL)
	}

	if prompt == "" {
		prompt = defaultPrompt
	}

	client := &http.Client{Timeout: 3 * time.Minute}

	resp, _, err := utils.POST[geminiResponse](ctx, client, geminiAPI+"?key="+apiKey, nil, map[string]any{
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
		return "", fmt.Errorf("gemini: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	if err := filesystem.UpdateUsage("gemini@gemini-3-flash-preview",
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

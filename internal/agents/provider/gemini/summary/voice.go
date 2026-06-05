package summary

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/record"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	model    = "gemini@gemini-3-flash-preview"
	endpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-flash-preview:generateContent"
)

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

func VoiceReply(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if len([]rune(text)) <= 320 {
		return text, nil
	}

	apiKey := strings.TrimSpace(keychain.Get("GEMINI_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required")
	}

	prompt := `Please rewrite the “complete final response” below into a natural Chinese spoken summary that can be read aloud directly.
Requirements:
- Summarize based on the full content, not just the first sentence or opening section, and do not copy the full content verbatim.
- Cover the main conclusion, important results, next steps, or limitations from the final response.
- Use a conversational but precise tone. Write one paragraph only. Do not use bullet points, Markdown, or XML/HTML tags.
- Keep it around 120 to 220 Chinese characters; if the original contains many details, it may be up to 280 characters.

Complete final response:
` + text

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, _, err := go_pkg_http.POST[data](ctx, nil, endpoint, map[string]string{"x-goog-api-key": apiKey}, map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]any{"text": prompt},
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
		if t := strings.TrimSpace(p.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "")), nil
}

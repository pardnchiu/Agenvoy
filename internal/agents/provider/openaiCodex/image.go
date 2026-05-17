package openaicodex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ImageOptions struct {
	Size        string
	Quality     string
	RefImageB64 string
	RefMime     string
}

type imageSSEEvent struct {
	Type string `json:"type"`
	Item *struct {
		Type          string `json:"type"`
		Result        string `json:"result"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"item,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *Agent) GenerateImage(ctx context.Context, prompt string, opts ImageOptions) (string, string, error) {
	auth, err := a.authHeader(ctx)
	if err != nil {
		return "", "", fmt.Errorf("a.authHeader: %w", err)
	}

	content := []map[string]any{
		{"type": "input_text", "text": prompt},
	}
	if opts.RefImageB64 != "" {
		mime := opts.RefMime
		if mime == "" {
			mime = "image/png"
		}
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": fmt.Sprintf("data:%s;base64,%s", mime, opts.RefImageB64),
		})
	}

	tool := map[string]any{"type": "image_generation"}
	if opts.Size != "" {
		tool["size"] = opts.Size
	}
	if opts.Quality != "" {
		tool["quality"] = opts.Quality
	}

	body := map[string]any{
		"model":        a.model,
		"instructions": "You are an image generation assistant. Use the image_generation tool to produce exactly one image matching the user's prompt. Do not respond with text.",
		"input": []map[string]any{
			{"role": "user", "content": content},
		},
		"tools":  []map[string]any{tool},
		"store":  false,
		"stream": true,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if a.token != nil && a.token.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", a.token.AccountID)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	// * SSE data lines carry full base64 PNG; 1024x1024 high quality ~6 MiB, leave headroom
	scanner.Buffer(make([]byte, 1<<20), 32<<20)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var ev imageSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		if ev.Error != nil {
			return "", "", fmt.Errorf("upstream %s: %s", ev.Error.Code, ev.Error.Message)
		}

		if ev.Type == "response.output_item.done" && ev.Item != nil && ev.Item.Type == "image_generation_call" {
			if ev.Item.Result == "" {
				return "", "", fmt.Errorf("image_generation_call missing result")
			}
			return ev.Item.Result, ev.Item.RevisedPrompt, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("scanner: %w", err)
	}
	return "", "", fmt.Errorf("no image_generation_call event in response")
}

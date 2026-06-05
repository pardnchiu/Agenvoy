package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registDownloadFile() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "download_file",
		AlwaysAllow: false,
		Concurrent:  true,
		Description: "Download a binary file from a URL to local disk. Use for tar.gz, images, archives, or any non-text content. For JSON/HTML/markdown use send_http_request or fetch_page(save=true).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full URL to download (must include http:// or https://).",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "Save path. Absolute path used as-is. Relative path joined under ~/.config/agenvoy/download/. Parent dir auto-created.",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout seconds (max 600). Use 300+ for large archives.",
					"default":     120,
				},
			},
			"required": []string{"url", "output_file"},
		},
		Handler: handleDownloadFile,
		Timeout: 10 * time.Minute,
	})
}

func handleDownloadFile(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
	var p struct {
		URL        string `json:"url"`
		OutputFile string `json:"output_file"`
		Timeout    int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}

	url := strings.TrimSpace(p.URL)
	out := strings.TrimSpace(p.OutputFile)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}
	if out == "" {
		return "", fmt.Errorf("output_file is required")
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(filesystem.DownloadDir, out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return "", fmt.Errorf("MkdirAll: %w", err)
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 120
	} else if timeout > 600 {
		timeout = 600
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("http.NewRequest: %w", err)
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("client.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	f, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("os.Create: %w", err)
	}
	size, copyErr := io.Copy(f, resp.Body)
	if closeErr := f.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(out)
		return "", fmt.Errorf("io.Copy: %w", copyErr)
	}

	result := map[string]any{
		"ok":          true,
		"url":         url,
		"output_file": out,
		"size_bytes":  size,
		"status_code": resp.StatusCode,
	}
	if sha := strings.TrimSpace(resp.Header.Get("X-Sha256")); sha != "" {
		result["sha256"] = sha
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(raw), nil
}

package utils

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	end := maxBytes
	for end > 0 && !utf8.Valid([]byte(s[:end])) {
		end--
	}
	return s[:end] + "\n...(truncated)"
}

func GET[T any](ctx context.Context, client *http.Client, api string, header map[string]string) (T, int, error) {
	var result T

	if client == nil {
		client = &http.Client{}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", api, nil)
	if err != nil {
		return result, 0, err
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return result, 0, err
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode

	if s, ok := any(&result).(*string); ok {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, statusCode, err
		}
		*s = string(b)
		return result, statusCode, nil
	}

	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "xml"):
		err = xml.NewDecoder(resp.Body).Decode(&result)
	default:
		err = json.NewDecoder(resp.Body).Decode(&result)
	}
	if err != nil {
		return result, statusCode, err
	}
	return result, statusCode, nil
}

func POST[T any](ctx context.Context, client *http.Client, api string, header map[string]string, body map[string]any, contentType string) (T, int, error) {
	var result T

	if contentType == "" {
		contentType = "json"
	}

	var req *http.Request
	var err error
	if contentType == "form" {
		requestBody := url.Values{}
		for k, v := range body {
			requestBody.Set(k, fmt.Sprint(v))
		}

		req, err = http.NewRequestWithContext(ctx, "POST", api, strings.NewReader(requestBody.Encode()))
		if err != nil {
			return result, 0, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		requestBody, err := json.Marshal(body)
		if err != nil {
			return result, 0, fmt.Errorf("failed to marshal body: %w", err)
		}

		req, err = http.NewRequestWithContext(ctx, "POST", api, strings.NewReader(string(requestBody)))
		if err != nil {
			return result, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")

	for k, v := range header {
		req.Header.Set(k, v)
	}

	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, 0, fmt.Errorf("failed to send: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode

	if statusCode < 200 || statusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return result, statusCode, fmt.Errorf("HTTP %d: %s", statusCode, strings.TrimSpace(string(b)))
	}

	if s, ok := any(&result).(*string); ok {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, statusCode, fmt.Errorf("failed to read: %w", err)
		}
		*s = string(b)
		return result, statusCode, nil
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result, statusCode, fmt.Errorf("failed to read: %w", err)
	}
	return result, statusCode, nil
}

const (
	projectName = "agenvoy"
)

type ConfigDirData struct {
	Home string
	Work string
	Dirs []string
}

func GetConfigDir(path ...string) (*ConfigDirData, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("os.UserHomeDir: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("os.Getwd: %w", err)
	}

	homeDir = strings.TrimSpace(filepath.Join(append([]string{homeDir, ".config", projectName}, path...)...))
	workDir = strings.TrimSpace(filepath.Join(append([]string{workDir, ".config", projectName}, path...)...))

	config := &ConfigDirData{
		Home: homeDir,
		Work: workDir,
		Dirs: []string{
			homeDir,
			workDir,
		},
	}

	err = os.MkdirAll(config.Home, 0755)
	if err != nil {
		return nil, fmt.Errorf("os.MkdirAll: %w", err)
	}

	return config, nil
}

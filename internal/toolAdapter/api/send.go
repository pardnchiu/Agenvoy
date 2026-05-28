package apiAdapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type ResponseData struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

var methods = []string{
	"GET", "POST", "PUT", "DELETE", "PATCH",
}

func buildMultipart(body map[string]any) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if fields, ok := body["fields"].(map[string]any); ok {
		for k, v := range fields {
			if err := w.WriteField(k, fmt.Sprint(v)); err != nil {
				return nil, "", fmt.Errorf("WriteField %q: %w", k, err)
			}
		}
	}

	if files, ok := body["files"].([]any); ok {
		for i, f := range files {
			fmap, ok := f.(map[string]any)
			if !ok {
				return nil, "", fmt.Errorf("files[%d] is not an object", i)
			}
			name, _ := fmap["name"].(string)
			path, _ := fmap["path"].(string)
			if name == "" || path == "" {
				return nil, "", fmt.Errorf("files[%d] requires name and path", i)
			}
			ct, _ := fmap["content_type"].(string)
			if ct == "" {
				ct = "application/octet-stream"
			}

			file, err := os.Open(path)
			if err != nil {
				return nil, "", fmt.Errorf("open %q: %w", path, err)
			}

			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, name, filepath.Base(path)))
			h.Set("Content-Type", ct)

			part, err := w.CreatePart(h)
			if err != nil {
				file.Close()
				return nil, "", fmt.Errorf("CreatePart %q: %w", name, err)
			}
			if _, err := io.Copy(part, file); err != nil {
				file.Close()
				return nil, "", fmt.Errorf("copy %q: %w", path, err)
			}
			file.Close()
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("Writer.Close: %w", err)
	}

	return &buf, w.FormDataContentType(), nil
}

func Send(ctx context.Context, api, method string, headers map[string]string, body map[string]any, contentType string, timeout int) (string, error) {
	if api == "" {
		return "", fmt.Errorf("url is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	if !slices.Contains(methods, method) {
		return "", fmt.Errorf("invalid method: %s", method)
	}

	if contentType == "" {
		contentType = "json"
	}

	if timeout <= 0 {
		timeout = 60
	} else if timeout > 300 {
		timeout = 300
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	switch method {
	case "GET", "DELETE":
		req, err = http.NewRequestWithContext(reqCtx, method, api, nil)
		if err != nil {
			return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
		}

	case "POST", "PUT", "PATCH":
		switch contentType {
		case "form":
			requestBody := url.Values{}
			for k, v := range body {
				requestBody.Set(k, fmt.Sprint(v))
			}

			req, err = http.NewRequestWithContext(reqCtx, method, api, strings.NewReader(requestBody.Encode()))
			if err != nil {
				return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		case "multipart":
			buf, ct, err := buildMultipart(body)
			if err != nil {
				return "", fmt.Errorf("buildMultipart: %w", err)
			}
			req, err = http.NewRequestWithContext(reqCtx, method, api, buf)
			if err != nil {
				return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
			}
			req.Header.Set("Content-Type", ct)

		default:
			requestBody, err := json.Marshal(body)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}

			req, err = http.NewRequestWithContext(reqCtx, method, api, strings.NewReader(string(requestBody)))
			if err != nil {
				return "", fmt.Errorf("http.NewRequestWithContext: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
		}
	}

	req.Header.Set("Accept", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("client.Do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("io.ReadAll: %w", err)
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	result := ResponseData{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       string(respBody),
	}

	output, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}

	return string(output), nil
}

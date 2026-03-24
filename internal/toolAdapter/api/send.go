package apiAdapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func Send(api, method string, headers map[string]string, body map[string]any, contentType string, timeout int) (string, error) {
	if api == "" {
		return "", fmt.Errorf("url is required")
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
		timeout = 30
	} else if timeout > 300 {
		timeout = 300
	}

	var req *http.Request
	var err error

	switch method {
	case "GET", "DELETE":
		req, err = http.NewRequest(method, api, nil)
		if err != nil {
			return "", fmt.Errorf("http.NewRequest: %w", err)
		}

	case "POST", "PUT", "PATCH":
		if contentType == "form" {
			requestBody := url.Values{}
			for k, v := range body {
				requestBody.Set(k, fmt.Sprint(v))
			}

			req, err = http.NewRequest(method, api, strings.NewReader(requestBody.Encode()))
			if err != nil {
				return "", fmt.Errorf("http.NewRequest: %w", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			requestBody, err := json.Marshal(body)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}

			req, err = http.NewRequest(method, api, strings.NewReader(string(requestBody)))
			if err != nil {
				return "", fmt.Errorf("http.NewRequest: %w", err)
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

package apiAdapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
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

var privateNets = []net.IPNet{
	{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
	{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
	{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
	{IP: net.IP{169, 254, 0, 0}, Mask: net.CIDRMask(16, 32)},
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
	{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},
	{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},
}

func checkSSRF(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url has no host")
	}
	if strings.EqualFold(u.Scheme, "file") || strings.EqualFold(u.Scheme, "ftp") {
		return fmt.Errorf("scheme %q not allowed", u.Scheme)
	}
	if slices.Contains(filesystem.NetWhiteList, strings.ToLower(host)) {
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup %q: %w", host, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("host %q resolves to non-routable address %s", host, ip)
		}
		for _, pn := range privateNets {
			if pn.Contains(ip) {
				return fmt.Errorf("host %q resolves to private address %s", host, ip)
			}
		}
	}
	return nil
}

func Send(ctx context.Context, api, method string, headers map[string]string, body map[string]any, contentType string, timeout int) (string, error) {
	if api == "" {
		return "", fmt.Errorf("url is required")
	}
	if err := checkSSRF(api); err != nil {
		return "", err
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

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
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

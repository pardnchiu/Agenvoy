package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/fetchPage"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registDownloadFile() {
	toolregister.Regist(toolregister.Def{
		Name:        "download_file",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  true,
		Description: `Downloads a URL onto local disk: a binary file as-is, or a web page rendered in a browser and saved as markdown, html or json.
Use for 下載 / 抓這個檔 / 存到本地 / 匯出網頁 — tar.gz, images, archives, installers, or a page to keep as a file.
Reading a page without saving → fetch_page; JSON or an API response → http_request.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Full URL to download (must include http:// or https://).",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "Save path. Absolute path used as-is. Relative path joined under the output directory. Parent dir auto-created.",
				},
				"render": map[string]any{
					"type":        "boolean",
					"description": "true renders a web page in a browser with the user's Chrome login and saves its content; false saves the raw HTTP response, for binaries and direct file links.",
					"default":     false,
				},
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "Saved format, used only with render=true: markdown for reading, html for the raw DOM, json for structured data.",
					"default":     "markdown",
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

func handleDownloadFile(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
	var p struct {
		URL        string `json:"url"`
		OutputFile string `json:"output_file"`
		Timeout    int    `json:"timeout"`
		Render     bool   `json:"render"`
		Type       string `json:"type"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
	}

	url := strings.TrimSpace(p.URL)
	out := strings.TrimSpace(p.OutputFile)
	if url == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "url", "required")
	}
	if out == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "output_file", "required")
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(filesystem.OutputDir(), out)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.MkdirAll", filepath.Dir(out), err)
	}

	if p.Render {
		outType, err := fetchPage.ParseOutputType(p.Type)
		if err != nil {
			return "", err
		}
		return fetchPage.SavePage(ctx, url, outType, out)
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
		return "", utilsmsg.Wrap(utilsmsg.Invalid, "http.NewRequest", "", err)
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "client.Do", "", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", utilsmsg.New(utilsmsg.Upstream, "http", "%d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	f, err := os.Create(out)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.Create", out, err)
	}
	size, copyErr := io.Copy(f, resp.Body)
	if closeErr := f.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(out)
		return "", utilsmsg.Wrap(utilsmsg.Internal, "io.Copy", out, copyErr)
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

	raw, err := utils.MarshalPlain(result)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "utils.MarshalPlain", "", err)
	}
	return string(raw), nil
}

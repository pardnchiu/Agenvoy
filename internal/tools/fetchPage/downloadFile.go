// * last modified: 26/11/19
package fetchPage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registDownloadFile() {
	toolregister.Regist(toolregister.Def{
		Name:          "download_file",
		SystemDefault: true,
		Concurrent:    true,
		Timeout:       10 * time.Minute,
		Description: `Downloads a URL onto local disk: a binary file as-is, or a web page rendered in a browser and saved as markdown, html or json.
Use for "download this", "grab this file", "save it locally", "export this page" — tar.gz, images, archives, installers, or a page to keep as a file.
Reading a page without saving → fetch_page; JSON or an API response → http_request.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "Full URL including http:// or https://; fetched with a plain GET, no custom headers.",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "Save path: absolute is used as-is; relative is joined under the output directory (not the working directory). Parent directories are created; an existing file is overwritten.",
				},
				"render": map[string]any{
					"type":        "boolean",
					"description": "true renders a web page in a browser with the user's Chrome login and saves its content; false saves the raw HTTP response, for binaries and direct file links.",
					"default":     false,
				},
				"output_type": map[string]any{
					"type":        "string",
					"enum":        []string{"markdown", "html", "json"},
					"description": "Saved format, only with render=true: markdown (readable text), html (raw DOM), json (structured data).",
					"default":     "markdown",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds for render=false, capped at 600; 300+ for large archives. Ignored with render=true.",
					"default":     120,
				},
			},
			"required": []string{
				"link",
				"output_file",
			},
		},
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Link       string `json:"link"`
				OutputFile string `json:"output_file"`
				Render     bool   `json:"render"`
				OutputType string `json:"output_type"`
				Timeout    int    `json:"timeout"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json: Unmarshal", "", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "link", "required")
			}

			outputFile := strings.TrimSpace(params.OutputFile)
			if outputFile == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "output_file", "required")
			}
			if !filepath.IsAbs(outputFile) {
				outputFile = filepath.Join(filesystem.OutputDir(), outputFile)
			}

			// NOTE: 下載網頁頁面直接使用 fetch 搭配 save
			if params.Render {
				outType, err := outputType(params.OutputType)
				if err != nil {
					return "", err
				}
				return handler(ctx, link, false, true, true, outType, defaultScroll, &outputFile, false)
			}

			// NOTE: 需搭配預設 timeput 避免卡死
			timeout := params.Timeout
			if timeout <= 0 {
				timeout = 120
			} else if timeout > 600 {
				timeout = 600
			}

			// NOTE: 下載檔案
			httpCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			httpResp, err := go_pkg_http.GETStream(httpCtx, &http.Client{Timeout: time.Duration(timeout) * time.Second}, link, nil)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_pkg_http: GETStream", link, err)
			}
			defer httpResp.Body.Close()

			if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
				// NOTE: 錯誤只讀取前 4kb 內容獲取關鍵訊息
				bytes, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4<<10))
				return "", utilsmsg.New(utilsmsg.Upstream, "http", "%d: %s", httpResp.StatusCode, strings.TrimSpace(string(bytes)))
			}

			size, err := go_pkg_filesystem.WriteStream(outputFile, httpResp.Body, 0644)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem: WriteStream", outputFile, err)
			}

			dic := map[string]any{
				"url":         link,
				"output_file": outputFile,
				"size_bytes":  size,
			}
			if sha := strings.TrimSpace(httpResp.Header.Get("X-Sha256")); sha != "" {
				dic["sha256"] = sha
			}

			raw, err := utils.MarshalPlain(dic)
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "internal/utils: MarshalPlain", "", err)
			}
			return string(raw), nil
		},
	})
}

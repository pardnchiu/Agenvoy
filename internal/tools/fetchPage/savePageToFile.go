package fetchPage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registSavePageToFile() {
	toolRegister.Regist(toolRegister.Def{
		Name: "save_page_to_file",
		Description: "Fetch a web page and save its content to a local file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"link": map[string]any{
					"type":        "string",
					"description": "The full URL to download (must include https://)",
				},
				"keep_links": map[string]any{
					"type":        "boolean",
					"description": "Keep hyperlinks from the same domain (useful for document research tasks that require recursively following subpages).",
					"default":     false,
				},
				"save_to": map[string]any{
					"type":        "string",
					"description": "Target file path to save. Use absolute path directly; relative paths will use ~/Downloads (if exists, preferred) or ~/.config/agenvoy/download/ as base. If not specified, will be saved to that directory automatically.",
					"default":     "",
				},
			},
			"required": []string{
				"link",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Link      string `json:"link"`
				KeepLinks bool   `json:"keep_links"`
				SaveTo    string `json:"save_to"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			link := strings.TrimSpace(params.Link)
			if link == "" {
				return "", fmt.Errorf("link is required")
			}

			saveTo := strings.TrimSpace(params.SaveTo)
			if saveTo == "" {
				saveTo = defaultDownloadPath(link)
			} else {
				abs, err := go_pkg_filesystem.AbsPath(filesystem.DownloadDir, saveTo, go_pkg_filesystem.AbsPathOption{HomeOnly: true})
				if err != nil {
					return "", fmt.Errorf("go_pkg_filesystem.AbsPath: %w", err)
				}
				saveTo = abs
			}
			return handler(link, params.KeepLinks, &saveTo)
		},
	})
}

func defaultDownloadPath(href string) string {
	name := "page"
	if u, err := url.Parse(href); err == nil {
		seg := strings.TrimSuffix(filepath.Base(u.Path), "/")
		if seg != "" && seg != "." {
			name = seg
		} else if u.Host != "" {
			name = u.Host
		}
	}
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return filepath.Join(filesystem.DownloadDir, name)
}

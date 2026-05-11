package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdatePage() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "update_page",
		AlwaysLoad:  true,
		Description: "Overwrite the rendered page for the current session (index.html under the session's page directory). Browser tabs viewing this session auto-reload. Pass the complete HTML; partial diffs unsupported.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "Complete HTML document. Must include <!DOCTYPE html>, <html>, <head>, <body>, </body>. Server injects the reload <script> immediately before </body>.",
				},
			},
			"required": []string{"content"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			content := params.Content
			if strings.TrimSpace(content) == "" {
				return "", fmt.Errorf("content is required")
			}

			sid := strings.TrimSpace(e.SessionID)
			if sid == "" {
				return "", fmt.Errorf("update_page requires an active session; current executor has no SessionID")
			}

			pageDir := filesystem.PagePath(sid)
			if err := go_pkg_filesystem.CheckDir(pageDir, true); err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.CheckDir: %w", err)
			}

			indexPath := filepath.Join(pageDir, "index.html")
			if err := go_pkg_filesystem.WriteFile(indexPath, content, 0644); err != nil {
				return "", fmt.Errorf("go_pkg_filesystem.WriteFile: %w", err)
			}

			return fmt.Sprintf("page updated: %s (browser will reload)", indexPath), nil
		},
	})
}

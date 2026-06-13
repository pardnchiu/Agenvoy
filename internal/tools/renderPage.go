package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const jarvisPageTTL = 3 * 24 * 3600 // 3 days

func init() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "render_page",
		AlwaysLoad:  true,
		AlwaysAllow: true,
		Description: "Render a complete HTML page for the current session. Browser tabs viewing this session auto-reload on completion. Pass the complete HTML document; partial diffs unsupported.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "Complete HTML document. Must include <!DOCTYPE html>, <html>, <head>, <body>, </body>.",
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
				return "", fmt.Errorf("render_page requires an active session")
			}

			ts := strconv.FormatInt(time.Now().UnixMilli(), 10)

			db := torii.DB(torii.DBJarvisPage)
			pageKey := sid + ":" + ts
			if err := db.Set(pageKey, content, torii.SetDefault, torii.TTL(jarvisPageTTL)); err != nil {
				return "", fmt.Errorf("torii.Set page: %w", err)
			}

			histKey := sid + ":history"
			entry, ok := db.Get(histKey)
			var list []string
			if ok {
				_ = json.Unmarshal([]byte(entry.Value()), &list)
			}
			list = append(list, ts)
			raw, _ := json.Marshal(list)
			_ = db.Set(histKey, string(raw), torii.SetDefault, nil)

			return fmt.Sprintf("page rendered: ts=%s (browser will reload)", ts), nil
		},
	})
}

package toolcache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func Register() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_recent_tool_call",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Description: `List cached tool calls (last 30 min) in this session. Only search_web, search_google_news, and fetch_page are cached. Call BEFORE re-invoking these three tools to check for a usable prior result. If a match exists, use read_tool_call(id) instead of re-running. All other tools are not cached — call them directly.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			list := List(e.SessionID)
			if len(list) == 0 {
				return "no cached tool calls in the last 30 minutes", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("%d cached tool call(s):\n\n", len(list)))
			for _, item := range list {
				m := int(item.Age.Minutes())
				s := int(item.Age.Seconds()) % 60
				sb.WriteString(fmt.Sprintf("%s | %s | %s | %dm%ds ago\n", item.ID, item.ToolName, item.Args, m, s))
			}
			sb.WriteString("\nUse read_tool_call(id) to retrieve any result.")
			return sb.String(), nil
		},
	})

	toolRegister.Regist(toolRegister.Def{
		Name:        "read_tool_call",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Description: "Retrieve the cached result of a previous tool call by its call_id (from list_recent_tool_call). Avoids re-executing the tool.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The call_id shown by list_recent_tool_call (e.g. call_eCMNfmm1590EejYzArArOJzc).",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			params.ID = strings.TrimSpace(params.ID)
			if params.ID == "" {
				return "", fmt.Errorf("id is required")
			}
			result, ok := Get(e.SessionID, params.ID)
			if !ok {
				return "", fmt.Errorf("id %s not found or expired (TTL 30 min)", params.ID)
			}
			return result, nil
		},
	})
}

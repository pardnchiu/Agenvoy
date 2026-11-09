package actionHistory

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	defaultListLimit  = 10
	defaultToolsLimit = 16
	listArgsRunes     = 256
)

func registChatHistory() {
	toolregister.Regist(toolregister.Def{
		Name:        "chat_history",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Returns this session's past runs: each run's objective, its tool calls with arguments and output, and the messages.
Use for 最近做了什麼 / 之前處理過這個嗎 / 那次查到什麼 / 剛剛的報告 / 那個工具回了什麼 / 我們之前聊過什麼, and before re-running a fetch, search or report the session may already have, so its output is reused instead.
Files in the workspace → read_files; file versions → file_history; tool failures → error_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "tool_list", "tool", "read", "search"},
					"description": "list: recent runs. tool_list: outputs worth reusing (network, subagent, generated media, mcp__/script_/ext_) across recent runs; check it before fetching anything. tool: one call's raw output. read: whole runs. search: past messages. Omitted: task_id + name → tool, task_id → read, keyword → search, otherwise list.",
					"default":     "list",
				},
				"task_id": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Ids from list or tool_list rows; read takes several, tool exactly one.",
				},
				"scope": map[string]any{
					"type":        "string",
					"enum":        []string{"reference", "full"},
					"description": "mode=read: reference keeps only the outputs tool_list covers and reduces other calls to names; full keeps every call's arguments and output.",
					"default":     "reference",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "mode=tool: the tool name exactly as in the tool_list row, e.g. 'fetch_page'.",
				},
				"args": map[string]any{
					"type":        "string",
					"description": "mode=tool: that row's arguments string; needed only when the run called the tool more than once.",
				},
				"keyword": map[string]any{
					"type":        "string",
					"description": "mode=search: the core noun to look for, e.g. 'redis TTL'.",
				},
				"match": map[string]any{
					"type":        "string",
					"enum":        []string{"semantic", "keyword"},
					"description": "mode=search: semantic matches meaning in recent messages; keyword matches exact text across the full history, archive included.",
					"default":     "semantic",
				},
				"time_range": map[string]any{
					"type":        "string",
					"enum":        slices.Sorted(maps.Keys(configs.TIME_RANGES)),
					"description": "mode=search: how far back to look.",
					"default":     defaultTimeRange,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "list: runs shown (default 10). tool_list: runs scanned (default 16). search: hits per source (default 8, max 32).",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode      string          `json:"mode"`
				Scope     string          `json:"scope"`
				TaskID    json.RawMessage `json:"task_id"`
				Keyword   string          `json:"keyword"`
				Match     string          `json:"match"`
				TimeRange string          `json:"time_range"`
				Name      string          `json:"name"`
				Args      string          `json:"args"`
				Limit     int             `json:"limit"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			params.Mode = strings.TrimSpace(params.Mode)
			taskIDs := parseTaskIDs(params.TaskID)

			full := false
			switch strings.TrimSpace(params.Scope) {
			case "", "reference":
			case "full":
				full = true
			default:
				return "", utilsmsg.New(utilsmsg.Invalid, "scope", "unknown %q; available: reference, full", params.Scope)
			}
			if params.Mode == "" {
				params.Mode = "list"
				switch {
				case len(taskIDs) > 0 && strings.TrimSpace(params.Name) != "":
					params.Mode = "tool"
				case len(taskIDs) > 0:
					params.Mode = "read"
				case strings.TrimSpace(params.Keyword) != "":
					params.Mode = "search"
				}
			}

			switch params.Mode {
			case "list":
				return list(e, params.Limit)
			case "tool_list":
				return toolList(e, params.Limit)
			case "tool":
				return toolContent(e, taskIDs, strings.TrimSpace(params.Name), params.Args)
			case "read":
				if len(taskIDs) == 0 {
					return "", utilsmsg.New(utilsmsg.Invalid, "task_id", "required when mode=read")
				}
				return read(e, taskIDs, full)
			case "search":
				return searchMessages(ctx, e, params.Keyword, params.Match, params.TimeRange, params.Limit)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: list, tool_list, tool, read, search", params.Mode)
		},
	})
}

func parseTaskIDs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		var one string
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil
		}
		list = []string{one}
	}

	ids := make([]string, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, id := range list {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

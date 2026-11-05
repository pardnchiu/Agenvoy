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
		Description: `This session's action log: each past run's objective, its tool calls with arguments and output, and the messages.
Use for 最近做了什麼 / 之前處理過這個嗎 / 那次到底做了什麼 / 那個工具回了什麼 / 上一個動作的 action log / 我們之前聊過什麼.
Before repeating a tool call, mode=list shows what past runs called with which arguments; mode=read returns their output — reuse instead of re-running.
Not the workspace — file listing never reaches them. Versions → file_history; failures → error_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "tool_list", "tool", "read", "search"},
					"description": "list: recent runs as a bare index — task_id, when it ended, what it was for. Match an objective here, then read that task_id. tool_list: the same calls flattened across the last 16 runs and reduced to the ones worth reusing (network fetches, subagent reports, generated media, mcp__/script_/ext_ payloads), each with its time, arguments and task_id — scan it before fetching anything to see whether this session already has it. tool: one call's raw output, addressed by task_id + name (+ args when that run called the tool more than once) — the cheapest way back to a page or search result already paid for. read: the full runs behind one or more task_ids, tool output untruncated. search: past messages, needs keyword. Omitted: task_id with name selects tool, task_id alone selects read, keyword selects search, otherwise list.",
					"default":     "list",
				},
				"task_id": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "mode=read: task ids taken from list rows; pass every run you want in one call. mode=tool: the single run holding the call you want. Required for both.",
				},
				"scope": map[string]any{
					"type":        "string",
					"enum":        []string{"reference", "full"},
					"description": "mode=read: reference keeps what costs money or time to obtain again — fetch_page, search_web, http_request, subagents reports, generate_image, download_file, and every mcp__/script_/ext_ payload — and reduces the rest to bare tool names. full includes every tool's arguments and output; use it to reconstruct what a run actually did.",
					"default":     "reference",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "mode=tool: the tool name exactly as it appears in a tool_list row (e.g. 'fetch_page', 'mcp__itrd__cycle').",
				},
				"args": map[string]any{
					"type":        "string",
					"description": "mode=tool: the arguments string from that same tool_list row, needed only when the run called the tool more than once; an unmatched value comes back with the ones it did use.",
				},
				"keyword": map[string]any{
					"type":        "string",
					"description": "mode=search: the core noun to look for (e.g. 'redis TTL', 'bwrap sandbox decision').",
				},
				"match": map[string]any{
					"type":        "string",
					"enum":        []string{"semantic", "keyword"},
					"description": "mode=search: semantic matches meaning in recent messages; keyword matches text across the full history including archive.",
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
					"description": "Rows to return, newest first; runs to scan when mode=tool_list (default 16); hit cap per source when mode=search. Ignored when mode=read.",
					"default":     defaultListLimit,
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
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: list, tools, read, search", params.Mode)
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

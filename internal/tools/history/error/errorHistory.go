package errorHistory

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec/memory"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	defaultSearchLimit = 4
	maxSearchLimit     = 16
)

func init() {
	registErrorHistory()
	registWriteError()
}

func registErrorHistory() {
	toolregister.Regist(toolregister.Def{
		Name:        "error_history",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Tool failures kept across sessions, each paired with the fix that resolved it.
Use for "this tool keeps failing", "how did we fix this error last time", before a second retry when no error hint was injected, and when a tool answers "no data: {hash}" (read that hash).
Storing a fix once it is confirmed working → write_error; a past run's own steps → chat_history; the full recovery loop → reasoning_guide(topics=[tool_error]).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"search", "read"},
					"description": "search: past records matching keyword, each with the fix that worked; apply it. read: one record by hash. Omitted: hash → read, otherwise search.",
					"default":     "search",
				},
				"keyword": map[string]any{
					"type":        "string",
					"description": "mode=search, required: tool name, error symptom or parameter trait, e.g. 'fetch_page 403', 'run_command network'.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "mode=search: max records to return; values above 16 are capped at 16.",
					"default":     defaultSearchLimit,
				},
				"hash": map[string]any{
					"type":        "string",
					"description": "mode=read: error hash, 8-char hex from a search row or a \"no data: {hash}\" reply, e.g. 'a1b2c3d4'.",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode    string `json:"mode"`
				Keyword string `json:"keyword"`
				Limit   int    `json:"limit"`
				Hash    string `json:"hash"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			params.Mode = strings.TrimSpace(params.Mode)
			params.Hash = strings.TrimSpace(params.Hash)
			if params.Mode == "" {
				params.Mode = "search"
				if params.Hash != "" {
					params.Mode = "read"
				}
			}

			switch params.Mode {
			case "search":
				keyword := strings.TrimSpace(params.Keyword)
				if keyword == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "keyword", "required when mode=search")
				}
				limit := params.Limit
				if limit <= 0 {
					limit = defaultSearchLimit
				}
				return memory.Search(ctx, "", keyword, min(limit, maxSearchLimit)), nil

			case "read":
				if params.Hash == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "hash", "required when mode=read")
				}
				return memory.Read(params.Hash), nil
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: search, read", params.Mode)
		},
	})
}

package errorHistory

import (
	"context"
	"encoding/json"
	"log/slog"
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
}

func registErrorHistory() {
	toolregister.Regist(toolregister.Def{
		Name:        "error_history",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Tool failures kept across sessions, each paired with the fix that resolved it.
Use for "this tool keeps failing", "how did we fix this error last time", before a second retry when no error hint was injected, when a tool answers "no data: {hash}" (read that hash), and once a non-trivial fix is confirmed working (write it).
A past run's own steps → chat_history; the full recovery loop → reasoning_guide(topics=[tool_error]).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"search", "read", "write"},
					"description": "search: past records matching keyword, each with the fix that worked; apply it. read: one record by hash. write: store an error and the fix that resolved it (needs tool_name, keywords, symptom, action, outcome). Omitted: hash → read, outcome → write, otherwise search.",
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
				"tool_name": map[string]any{
					"type":        "string",
					"description": "mode=write, required: the tool that produced the error, e.g. 'fetch_page'.",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "mode=write, required: lookup keywords a later search would use, e.g. ['fetch_page', '403', 'cloudflare'].",
				},
				"symptom": map[string]any{
					"type":        "string",
					"description": "mode=write, required: what the tool returned or failed on.",
				},
				"cause": map[string]any{
					"type":        "string",
					"description": "mode=write: root cause, once confirmed.",
					"default":     "",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "mode=write, required: what was done, e.g. 'retried with English keyword', 'fell back to search_web'.",
				},
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"resolved", "failed", "abandoned"},
					"description": "mode=write, required. resolved: the fix worked, the record is stored. failed / abandoned: accepted but discarded, nothing is stored.",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode     string   `json:"mode"`
				Keyword  string   `json:"keyword"`
				Limit    int      `json:"limit"`
				Hash     string   `json:"hash"`
				ToolName string   `json:"tool_name"`
				Keywords []string `json:"keywords"`
				Symptom  string   `json:"symptom"`
				Cause    string   `json:"cause"`
				Action   string   `json:"action"`
				Outcome  string   `json:"outcome"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			params.Mode = strings.TrimSpace(params.Mode)
			params.Hash = strings.TrimSpace(params.Hash)
			params.Action = strings.TrimSpace(params.Action)
			params.Outcome = strings.TrimSpace(params.Outcome)
			if params.Mode == "" {
				params.Mode = "search"
				switch {
				case params.Hash != "":
					params.Mode = "read"
				case params.Outcome != "":
					params.Mode = "write"
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

			case "write":
				record := memory.Record{
					ToolName: strings.TrimSpace(params.ToolName),
					Keywords: params.Keywords,
					Symptom:  strings.TrimSpace(params.Symptom),
					Cause:    strings.TrimSpace(params.Cause),
					Action:   params.Action,
					Outcome:  params.Outcome,
				}
				if err := requireRecord(record); err != nil {
					return "", err
				}
				saveCtx := context.WithoutCancel(ctx)
				go func() {
					if _, err := memory.Save(saveCtx, e.SessionID, record); err != nil {
						slog.Debug("error_history: memory.Save",
							slog.String("tool", record.ToolName),
							slog.String("error", err.Error()))
					}
				}()
				return "recorded", nil
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: search, read, write", params.Mode)
		},
	})
}

func requireRecord(r memory.Record) error {
	switch {
	case r.ToolName == "":
		return utilsmsg.New(utilsmsg.Invalid, "tool_name", "required when mode=write")
	case len(r.Keywords) == 0:
		return utilsmsg.New(utilsmsg.Invalid, "keywords", "required when mode=write")
	case r.Symptom == "":
		return utilsmsg.New(utilsmsg.Invalid, "symptom", "required when mode=write")
	case r.Action == "":
		return utilsmsg.New(utilsmsg.Invalid, "action", "required when mode=write")
	case r.Outcome == "":
		return utilsmsg.New(utilsmsg.Invalid, "outcome", "required when mode=write")
	}
	return nil
}

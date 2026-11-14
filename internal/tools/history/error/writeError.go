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

func registWriteError() {
	toolregister.Regist(toolregister.Def{
		Name:        "write_error",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Stores a tool failure together with the fix that resolved it, so a later error_history search finds it; returns "recorded".
Use once a non-trivial fix is confirmed working, and when a failed tool result's next: hint asks for it.
Looking up past failures and their fixes → error_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{
					"type":        "string",
					"description": "The tool that produced the error, e.g. 'fetch_page'.",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Lookup keywords a later search would use, e.g. ['fetch_page', '403', 'cloudflare'].",
				},
				"symptom": map[string]any{
					"type":        "string",
					"description": "What the tool returned or failed on.",
				},
				"cause": map[string]any{
					"type":        "string",
					"description": "Root cause, once confirmed.",
					"default":     "",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "What was done, e.g. 'retried with English keyword', 'fell back to search_web'.",
				},
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"resolved", "failed", "abandoned"},
					"description": "resolved: the fix worked, the record is stored. failed / abandoned: accepted but discarded, nothing is stored.",
				},
			},
			"required": []string{"tool_name", "keywords", "symptom", "action", "outcome"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ToolName string   `json:"tool_name"`
				Keywords []string `json:"keywords"`
				Symptom  string   `json:"symptom"`
				Cause    string   `json:"cause"`
				Action   string   `json:"action"`
				Outcome  string   `json:"outcome"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			record := memory.Record{
				ToolName: strings.TrimSpace(params.ToolName),
				Keywords: params.Keywords,
				Symptom:  strings.TrimSpace(params.Symptom),
				Cause:    strings.TrimSpace(params.Cause),
				Action:   strings.TrimSpace(params.Action),
				Outcome:  strings.TrimSpace(params.Outcome),
			}
			if err := requireRecord(record); err != nil {
				return "", err
			}
			saveCtx := context.WithoutCancel(ctx)
			go func() {
				if _, err := memory.Save(saveCtx, e.SessionID, record); err != nil {
					slog.Debug("write_error: memory.Save",
						slog.String("tool", record.ToolName),
						slog.String("error", err.Error()))
				}
			}()
			return "recorded", nil
		},
	})
}

func requireRecord(r memory.Record) error {
	switch {
	case r.ToolName == "":
		return utilsmsg.New(utilsmsg.Invalid, "tool_name", "required")
	case len(r.Keywords) == 0:
		return utilsmsg.New(utilsmsg.Invalid, "keywords", "required")
	case r.Symptom == "":
		return utilsmsg.New(utilsmsg.Invalid, "symptom", "required")
	case r.Action == "":
		return utilsmsg.New(utilsmsg.Invalid, "action", "required")
	case r.Outcome == "":
		return utilsmsg.New(utilsmsg.Invalid, "outcome", "required")
	}
	return nil
}

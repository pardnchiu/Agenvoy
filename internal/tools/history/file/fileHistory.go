package fileHistory

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	maxHistoryRows = 24
	defaultRows    = 3
)

func registFileHistory() {
	toolregister.Regist(toolregister.Def{
		Name:        "file_history",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Recorded versions of files the tools changed: when each changed, which task changed it and its objective, and the stored content diffed against the file on disk.
Use for "how many times has this file changed", "what was the previous version", "what changed since then", "what was in the part that got deleted".
Putting a version back → edit_file(mode=restore); what a whole run did → chat_history.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "read"},
					"description": "list: recorded versions with version id, time, task and objective, newest first. read: only the newest recorded version of each file, diffed against what is on disk now. Omitted: paths → read, otherwise list.",
					"default":     "list",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "mode=list: one file to look back over; relative paths resolve against the work directory, e.g. '/Users/me/notes.md', '~/notes.md', 'src/main.go'. Omit to list changes to every file, narrowed by task_id / from / to.",
				},
				"paths": map[string]any{
					"type":        "array",
					"description": "mode=read: files to compare, every path batched into one call; mode=list accepts a single entry here as path.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"task_id": map[string]any{
					"type":        "string",
					"description": "mode=list: only what one task changed; a task id from chat_history, or 'current' for the task running now.",
				},
				"from": map[string]any{
					"type":        "string",
					"description": "mode=list: inclusive local start time: '2026-08-13' (that day at 00:00), '2026-08-13 15:04', '2026-08-13 15:04:05', or RFC3339.",
				},
				"to": map[string]any{
					"type":        "string",
					"description": "mode=list: exclusive local end time, same formats as from; '2026-08-14' ends just before that day starts.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "mode=list: rows to return, newest first; values above 24 are capped at 24.",
					"default":     defaultRows,
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode   string   `json:"mode"`
				Path   string   `json:"path"`
				Paths  []string `json:"paths"`
				TaskID string   `json:"task_id"`
				From   string   `json:"from"`
				To     string   `json:"to"`
				Limit  int      `json:"limit"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			params.Mode = strings.TrimSpace(params.Mode)
			if params.Mode == "" {
				params.Mode = "list"
				if len(params.Paths) > 0 {
					params.Mode = "read"
				}
			}

			switch params.Mode {
			case "list":
				path := strings.TrimSpace(params.Path)
				if path == "" && len(params.Paths) == 1 {
					path = params.Paths[0]
				}
				if path == "" && len(params.Paths) > 1 {
					return "", utilsmsg.New(utilsmsg.Invalid, "paths", "mode=list takes one path; pass them as paths with mode=read, or call once per file")
				}
				return list(ctx, e, path, params.TaskID, params.From, params.To, params.Limit)
			case "read":
				paths := params.Paths
				if len(paths) == 0 && strings.TrimSpace(params.Path) != "" {
					paths = []string{params.Path}
				}
				if len(paths) == 0 {
					return "", utilsmsg.New(utilsmsg.Invalid, "paths", "required when mode=read")
				}
				return read(ctx, e, paths)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown %q; available: list, read", params.Mode)
		},
	})
}

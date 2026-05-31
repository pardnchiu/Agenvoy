package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_task",
		Description: "List one-shot tasks scheduled in the current session from tasks.json. No parameters. Returns JSON array of {at, session_id, skill} filtered to the caller's session_id; empty array when nothing is scheduled in this session.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			tasks, err := runtime.LoadTasks()
			if err != nil {
				return "", fmt.Errorf("LoadTasks: %w", err)
			}
			sid := ""
			if e != nil {
				sid = strings.TrimSpace(e.SessionID)
			}
			filtered := make([]runtime.TaskEntry, 0, len(tasks))
			for _, t := range tasks {
				if strings.TrimSpace(t.SessionID) != sid {
					continue
				}
				filtered = append(filtered, t)
			}
			if len(filtered) == 0 {
				return "[]", nil
			}

			raw, err := json.Marshal(filtered)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

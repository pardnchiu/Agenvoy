package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_task",
		Description: "List currently scheduled one-shot tasks from tasks.json. No parameters. Returns JSON array of {at, session_id, skill}; empty array when nothing is scheduled.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			tasks, err := runtime.LoadTasks()
			if err != nil {
				return "", fmt.Errorf("LoadTasks: %w", err)
			}
			if len(tasks) == 0 {
				return "[]", nil
			}

			buf, err := json.Marshal(tasks)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(buf), nil
		},
	})
}

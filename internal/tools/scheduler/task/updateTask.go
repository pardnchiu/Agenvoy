package taskTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdateTask() {
	toolRegister.Regist(toolRegister.Def{
		Name: "update_task",
		Description: `
Reschedule an existing one-shot task by replacing its run time.
Use to change when a task fires without touching its script or other settings; modify script body via update_script.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Task ID returned by list_tasks.",
				},
				"at": map[string]any{
					"type":        "string",
					"description": "Run time as duration ('+5m', '+1h30m'), today's clock time ('15:04'), date+time ('2006-01-02 15:04'), or RFC3339.",
				},
			},
			"required": []string{
				"id",
				"at",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
				At string `json:"at"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			id := strings.TrimSpace(params.ID)
			if id == "" {
				return "", fmt.Errorf("id is required")
			}

			at := strings.TrimSpace(params.At)
			if at == "" {
				return "", fmt.Errorf("at is required")
			}

			if err := tasks.Update(scheduler.Get(), id, at); err != nil {
				return "", err
			}
			return fmt.Sprintf("updated task: %s with cronExpression: %s", id, at), nil
		},
	})
}

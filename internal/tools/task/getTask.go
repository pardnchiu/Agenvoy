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

func registGetTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "get_task",
		ReadOnly: true,
		Description: `
Inspect a one-shot task's status and run details.
Use to verify scheduling after add_task or to diagnose a failed run.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Task ID returned by add_task or list_tasks.",
				},
			},
			"required": []string{
				"id",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			id := strings.TrimSpace(params.ID)
			if id == "" {
				return "", fmt.Errorf("id is required")
			}

			item, ok := tasks.GetTask(scheduler.Get(), id)
			if !ok {
				return "", fmt.Errorf("not found: %s", id)
			}

			lines := []string{
				fmt.Sprintf("id: %s", item.ID),
				fmt.Sprintf("status: %s", item.Status),
				fmt.Sprintf("scheduled_at: %s", item.At.Local().Format("2006-01-02 15:04:05")),
				fmt.Sprintf("script: %s", item.Script),
			}
			if item.StartedAt != nil {
				lines = append(lines, fmt.Sprintf("started_at: %s", item.StartedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if item.FinishedAt != nil {
				lines = append(lines, fmt.Sprintf("finished_at: %s", item.FinishedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if item.Output != "" {
				lines = append(lines, fmt.Sprintf("output: %s", item.Output))
			}
			if item.Err != "" {
				lines = append(lines, fmt.Sprintf("error: %s", item.Err))
			}
			return strings.Join(lines, "\n"), nil
		},
	})

}

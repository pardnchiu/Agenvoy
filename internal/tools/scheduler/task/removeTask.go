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

func registRemoveTask() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_task",
		Description: "Cancel a pending one-shot task by ID and delete its script file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Task ID returned by list_tasks.",
				},
			},
			"required": []string{
				"id",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
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

			if err := tasks.Delete(scheduler.Get(), id); err != nil {
				return "", err
			}
			return fmt.Sprintf("removed task: %s", id), nil
		},
	})

}

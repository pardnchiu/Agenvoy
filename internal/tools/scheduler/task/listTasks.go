package taskTools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListTools() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "list_tasks",
		ReadOnly: true,
		Description: `
List all pending one-shot tasks.
Use to discover task IDs needed by get_task, update_task, or remove_task.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			results, err := tasks.ListTasks(scheduler.Get())
			if err != nil {
				return "", err
			}

			if len(results) == 0 {
				return "please add tash first", nil
			}
			return strings.Join(results, ","), nil
		},
	})
}

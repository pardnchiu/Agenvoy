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
		Name:        "list_tasks",
		ReadOnly:    true,
		Description: "列出所有待執行的一次性定時任務，每行一筆，格式為 `{id} {time} {script}`。id 為移除時所需的識別碼。",
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
				return "no onetime tasks", nil
			}
			return strings.Join(results, "\n"), nil
		},
	})

}

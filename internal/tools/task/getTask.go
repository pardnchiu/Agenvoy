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
		Name:        "get_task",
		ReadOnly:    true,
		Description: "查詢指定 ID 的一次性任務狀態（pending/running/completed/failed）、執行時間、輸出結果與錯誤訊息。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "任務 ID（由 add_task 或 list_tasks 回傳）",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			t, ok := tasks.GetTask(scheduler.Get(), params.ID)
			if !ok {
				return "", fmt.Errorf("not found: %s", params.ID)
			}
			lines := []string{
				fmt.Sprintf("id: %s", t.ID),
				fmt.Sprintf("status: %s", t.Status),
				fmt.Sprintf("scheduled_at: %s", t.At.Local().Format("2006-01-02 15:04:05")),
				fmt.Sprintf("script: %s", t.Script),
			}
			if t.StartedAt != nil {
				lines = append(lines, fmt.Sprintf("started_at: %s", t.StartedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if t.FinishedAt != nil {
				lines = append(lines, fmt.Sprintf("finished_at: %s", t.FinishedAt.Local().Format("2006-01-02 15:04:05")))
			}
			if t.Output != "" {
				lines = append(lines, fmt.Sprintf("output: %s", t.Output))
			}
			if t.Err != "" {
				lines = append(lines, fmt.Sprintf("error: %s", t.Err))
			}
			return strings.Join(lines, "\n"), nil
		},
	})

}

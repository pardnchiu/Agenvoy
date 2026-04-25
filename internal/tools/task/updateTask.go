package taskTools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdateTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "update_task",
		Description: "修改已存在的一次性任務的執行時間，不刪除腳本、不影響其他設定。若要修改腳本內容請用 update_script。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "要修改的任務 ID（由 list_tasks 回傳）",
				},
				"at": map[string]any{
					"type":        "string",
					"description": "新的執行時間，支援：+5m、+1h30m、15:04、2006-01-02 15:04、RFC3339",
				},
			},
			"required": []string{"id", "at"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
				At string `json:"at"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if err := tasks.Update(scheduler.Get(), params.ID, params.At); err != nil {
				return "", err
			}
			return fmt.Sprintf("task %s updated: scheduled at %s", params.ID, params.At), nil
		},
	})

}

package cronTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registGetCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "get_cron",
		ReadOnly:    true,
		Description: "查詢指定 ID 的 cron 任務設定與最後一次執行狀態（completed/failed）、執行時間、輸出結果與錯誤訊息。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "cron 任務 ID（由 add_cron 或 list_crons 回傳）",
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
			c, r, ok := crons.GetCron(scheduler.Get(), params.ID)
			if !ok {
				return "", fmt.Errorf("not found: %s", params.ID)
			}
			lines := []string{
				fmt.Sprintf("id: %s", c.ID),
				fmt.Sprintf("expression: %s", c.Expression),
				fmt.Sprintf("script: %s", c.Script),
			}
			if r != nil {
				if r.RunAt != nil {
					lines = append(lines, fmt.Sprintf("last_run_at: %s", r.RunAt.Local().Format("2006-01-02 15:04:05")))
				}
				lines = append(lines, fmt.Sprintf("last_run_status: %s", r.Status))
				if r.Output != "" {
					lines = append(lines, fmt.Sprintf("last_run_output: %s", r.Output))
				}
				if r.Err != "" {
					lines = append(lines, fmt.Sprintf("last_run_err: %s", r.Err))
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})

}

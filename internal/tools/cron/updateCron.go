package cronTools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registUpdateCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "update_cron",
		Description: "修改已存在的 cron 任務的執行時間表達式，不刪除腳本、不影響其他設定。若要修改腳本內容請用 update_script。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "要修改的 cron 任務 ID（由 list_crons 回傳）",
				},
				"cron_expr": map[string]any{
					"type":        "string",
					"description": "新的 cron 表達式，5 個欄位：`{分} {時} {日} {月} {週}`",
				},
			},
			"required": []string{"id", "cron_expr"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID       string `json:"id"`
				CronExpr string `json:"cron_expr"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if err := crons.Update(scheduler.Get(), params.ID, params.CronExpr); err != nil {
				return "", err
			}
			return fmt.Sprintf("cron %s updated: %s", params.ID, params.CronExpr), nil
		},
	})
}

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

func registRemoveCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_cron",
		Description: "移除指定 ID 的重複性 cron 任務。**僅限使用者明確要求刪除排程時才可呼叫，禁止在建立排程流程中主動呼叫。** 若使用者未指定 ID：先呼叫 list_crons 取得列表，若只有一筆直接移除，若有多筆必須將列表回覆使用者並等待確認。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "任務 ID（由 list_crons 回傳的第一欄）",
				},
			},
			"required": []string{"id"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if err := crons.Delete(scheduler.Get(), params.ID); err != nil {
				return "", err
			}
			return fmt.Sprintf("cron task %s removed", params.ID), nil
		},
	})
}

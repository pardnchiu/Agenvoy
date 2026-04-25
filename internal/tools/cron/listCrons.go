package cronTools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListCrons() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_crons",
		ReadOnly:    true,
		Description: "列出所有目前啟用中的重複性 cron 任務，每行一筆，格式為 `{index}. {cron_expr} {script} [{channel_id}]`。index 為移除時所需的編號。",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			results, err := crons.List(scheduler.Get())
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "no cron tasks", nil
			}
			return strings.Join(results, "\n"), nil
		},
	})
}

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

func registUpdateCron() {
	toolRegister.Regist(toolRegister.Def{
		Name: "update_cron",
		Description: `
Retime an existing cron task by replacing its expression.
Use to change when a schedule fires without touching its script or other settings; modify script body via update_script.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Cron task ID returned by list_crons.",
				},
				"cron_expression": map[string]any{
					"type":        "string",
					"description": "Standard 5-field cron expression '{min} {hour} {day} {month} {weekday}' (e.g. '* * * * *', '0 9 * * 1', '*/5 * * * *').",
				},
			},
			"required": []string{
				"id",
				"cron_expression",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID             string `json:"id"`
				CronExpression string `json:"cron_expression"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			id := strings.TrimSpace(params.ID)
			if id == "" {
				return "", fmt.Errorf("id is required")
			}

			cronExpression := strings.TrimSpace(params.CronExpression)
			if cronExpression == "" {
				return "", fmt.Errorf("cron_expression is required")
			}

			if err := crons.Update(scheduler.Get(), id, cronExpression); err != nil {
				return "", err
			}
			return fmt.Sprintf("updated cron: %s with cronExpression: %s", id, cronExpression), nil
		},
	})
}

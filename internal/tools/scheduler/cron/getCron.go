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
		Name:     "get_cron",
		ReadOnly: true,
		Description: `
Inspect a cron task's configuration and its last run state.
Use to verify schedule wiring after add_cron or to diagnose a failed run.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Cron task ID returned by add_cron or list_crons.",
				},
			},
			"required": []string{
				"id",
			},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
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

			item, result, ok := crons.GetCron(scheduler.Get(), id)
			if !ok {
				return "", fmt.Errorf("not found: %s", id)
			}

			lines := []string{
				fmt.Sprintf("id: %s", item.ID),
				fmt.Sprintf("expression: %s", item.Expression),
				fmt.Sprintf("script: %s", item.Script),
			}
			if result != nil {
				if result.RunAt != nil {
					lines = append(lines, fmt.Sprintf("last_run_at: %s", result.RunAt.Local().Format("2006-01-02 15:04:05")))
				}
				lines = append(lines, fmt.Sprintf("last_run_status: %s", result.Status))
				if result.Output != "" {
					lines = append(lines, fmt.Sprintf("last_run_output: %s", result.Output))
				}
				if result.Err != "" {
					lines = append(lines, fmt.Sprintf("last_run_err: %s", result.Err))
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})

}

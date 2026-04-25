package cronTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registAddCron() {
	toolRegister.Regist(toolRegister.Def{
		Name: "add_cron",
		Description: `
Schedule a recurring task driven by a standard cron expression.
Run a script on a repeating timetable (every minute, hourly, daily, weekly).
Requires a script filename returned by write_script; multiple cron entries may share one script.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cron_expr": map[string]any{
					"type":        "string",
					"description": "Standard 5-field cron expression '{min} {hour} {day} {month} {weekday}' (e.g. '* * * * *', '0 9 * * 1', '*/5 * * * *').",
				},
				"script": map[string]any{
					"type":        "string",
					"description": "Filename returned by write_script (e.g. 'backup_1741569300.sh').",
				},
				"channel_id": map[string]any{
					"type":        "string",
					"description": "Discord channel ID to post each run's output to. Optional.",
				},
			},
			"required": []string{
				"cron_expression",
				"script",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				CronExpression string `json:"cron_expression"`
				Script         string `json:"script"`
				ChannelID      string `json:"channel_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			cronExpression := strings.TrimSpace(params.CronExpression)
			if cronExpression == "" {
				return "", fmt.Errorf("cron_expression is required")
			}

			script := strings.TrimSpace(params.Script)
			if script == "" {
				return "", fmt.Errorf("script is required")
			}

			channelId := strings.TrimSpace(params.ChannelID)
			if channelId == "" {
				if id, err := session.GetChannelID(e.SessionID); err == nil {
					channelId = id
				}
			}

			sched := scheduler.Get()
			if sched == nil {
				msg, err := crons.AddToFile(cronExpression, script, channelId)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("added cron to file: %s", msg), nil
			}
			return crons.Add(sched, cronExpression, script, channelId)
		},
	})

}

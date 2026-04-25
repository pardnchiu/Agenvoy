package taskTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/tasks"
	"github.com/pardnchiu/agenvoy/internal/session"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registerAddTask() {
	toolRegister.Regist(toolRegister.Def{
		Name: "add_task",
		Description: `
Schedule a one-shot task to run a script at a specific time.
After firing, the task is auto-removed from the schedule and its script file is deleted.
Requires a script filename returned by write_script.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"at": map[string]any{
					"type":        "string",
					"description": "Run time as duration ('+5m', '+1h30m'), today's clock time ('15:04'), date+time ('2006-01-02 15:04'), or RFC3339.",
				},
				"script": map[string]any{
					"type":        "string",
					"description": "Filename returned by write_script (e.g. 'open_pardn_io_1741569300.sh').",
				},
				"channel_id": map[string]any{
					"type":        "string",
					"description": "Discord channel ID to post the script's output to.",
					"default":     "",
				},
			},
			"required": []string{
				"at",
				"script",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				At        string `json:"at"`
				Script    string `json:"script"`
				ChannelID string `json:"channel_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			at := strings.TrimSpace(params.At)
			if at == "" {
				return "", fmt.Errorf("at is required")
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
				msg, err := tasks.AddToFile(at, script, channelId)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("added task to file: %s", msg), nil
			}
			return tasks.Add(sched, at, script, channelId)
		},
	})

}

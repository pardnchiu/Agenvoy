package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registPatchTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_task",
		Description: "Reschedule a one-shot task by skill name. Updates the fire time of the existing entry without changing the bound skill.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill full name (e.g. 'meeting-reminder-a3f9b2c1') used to locate the task entry.",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "New fire time: '+5m' / '+1h30m' (relative), '15:04' (today clock), '2006-01-02 15:04' (local datetime), or RFC3339.",
				},
			},
			"required": []string{"skill_name", "time"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				SkillName string `json:"skill_name"`
				Time      string `json:"time"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}

			at, err := parseTime(params.Time)
			if err != nil {
				return "", err
			}
			if !at.After(time.Now()) {
				return "", fmt.Errorf("already gone: %s", at.Local().Format("2006-01-02 15:04:05"))
			}

			patched, err := runtime.PatchTask(skill, at.UTC())
			if err != nil {
				return "", err
			}
			if patched == 0 {
				return fmt.Sprintf("no task found for skill %q", skill), nil
			}
			return fmt.Sprintf("patched %d task(s) for skill %q; new fire time: %s",
				patched, skill, at.Local().Format("2006-01-02 15:04:05")), nil
		},
	})
}

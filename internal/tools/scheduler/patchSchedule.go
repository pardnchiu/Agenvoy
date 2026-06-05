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

func registPatchSchedule() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_schedule",
		Description: "Reschedule a task or cron by skill name. Updates only the time/expression of the existing entry without changing the bound skill.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"enum":        []string{"task", "cron"},
					"description": "Schedule type to patch: 'task' for one-shot, 'cron' for recurring.",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill full name (e.g. 'meeting-reminder-a3f9b2c1') used to locate the entry.",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "For task: new fire time '+5m' / '+1h30m' (relative), '15:04' (today clock), '2006-01-02 15:04' (local datetime), or RFC3339. For cron: new 5-field expression '{min} {hour} {dom} {mon} {dow}'.",
				},
			},
			"required": []string{"target", "skill_name", "time"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Target    string `json:"target"`
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

			switch strings.ToLower(strings.TrimSpace(params.Target)) {
			case "task":
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

			case "cron":
				expression := strings.TrimSpace(params.Time)
				if len(strings.Fields(expression)) != 5 {
					return "", fmt.Errorf("expression must be 5 fields '{min} {hour} {dom} {mon} {dow}' (got %q)", expression)
				}
				patched, err := runtime.PatchCron(skill, expression)
				if err != nil {
					return "", err
				}
				if patched == 0 {
					return fmt.Sprintf("no cron found for skill %q", skill), nil
				}
				return fmt.Sprintf("patched %d cron(s) for skill %q; new expression: %s",
					patched, skill, expression), nil

			default:
				return "", fmt.Errorf("target must be 'task' or 'cron' (got %q)", params.Target)
			}
		},
	})
}

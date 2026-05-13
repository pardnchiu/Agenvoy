package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registAddCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_cron",
		Description: "Low-level cron-binding for an existing scheduler skill. DO NOT call directly when the user asks for a new recurring task — use the scheduler-skill-creator skill instead (it creates the skill body then calls this tool). Use this tool directly ONLY when the skill_name already exists under ~/.config/agenvoy/skills/scheduler/<name>/ and the user is rebinding/changing the schedule.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"time": map[string]any{
					"type":        "string",
					"description": "Standard 5-field cron expression '{min} {hour} {dom} {mon} {dow}' (e.g. '*/5 * * * *', '0 9 * * *', '30 8 * * 1').",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill short name (e.g. 'daily-hn-digest'). No 'scheduler-' prefix. Must already exist under ~/.config/agenvoy/skills/scheduler/<name>/.",
				},
			},
			"required": []string{"time", "skill_name"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Time      string `json:"time"`
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			expression := strings.TrimSpace(params.Time)
			if len(strings.Fields(expression)) != 5 {
				return "", fmt.Errorf("expression must be 5 fields '{min} {hour} {dom} {mon} {dow}' (got %q)", expression)
			}

			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}
			if !filesystem.ScheduleSkillExists(skill) {
				return "", fmt.Errorf("skill %q not found under %s. If the user is requesting a new recurring task, you should be running the scheduler-skill-creator skill (which handles create + bind in one flow), not calling add_cron directly. If you have just created the skill, call add_cron again with the same arguments now to complete the binding", skill, filesystem.ScheduleSkillPath(skill))
			}

			entry := runtime.CronEntry{
				Expression: expression,
				SessionID:  strings.TrimSpace(e.SessionID),
				Skill:      skill,
			}
			if err := runtime.AppendCron(entry); err != nil {
				return "", err
			}

			return fmt.Sprintf("cron scheduled: %s for %s", expression, skill), nil
		},
	})
}

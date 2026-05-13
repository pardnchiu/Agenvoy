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
		Description: "Internal cron-binding called by the scheduler-skill-creator skill flow. LLM must NOT call directly — every scheduler skill uses a hash-suffixed name that only scheduler-skill-creator generates, so any hand-made skill_name will fail.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"time": map[string]any{
					"type":        "string",
					"description": "Standard 5-field cron expression '{min} {hour} {dom} {mon} {dow}' (e.g. '*/5 * * * *', '0 9 * * *', '30 8 * * 1').",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Hashed scheduler skill name '<short>-<hash8>' produced by scheduler-skill-creator (no 'scheduler-' prefix). Never hand-craft this value.",
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
				return "", fmt.Errorf("skill %q not found under %s. add_cron is an internal binding called by the scheduler-skill-creator skill flow. Run scheduler-skill-creator skill which generates a hashed skill name and binds the schedule in one flow. Do not call add_cron with a hand-made name", skill, filesystem.ScheduleSkillPath(skill))
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

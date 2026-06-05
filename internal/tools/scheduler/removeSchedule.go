package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveSchedule() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_schedule",
		Description: "Cancel a scheduled task or cron by its skill name (full name including hash suffix). The bound scheduler skill directory is moved to .Trash/.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"enum":        []string{"task", "cron"},
					"description": "Schedule type to remove: 'task' for one-shot, 'cron' for recurring.",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill full name (e.g. 'meeting-reminder-a3f9b2c1').",
				},
			},
			"required": []string{"target", "skill_name"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Target    string `json:"target"`
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			skillName := strings.TrimSpace(params.SkillName)
			if skillName == "" {
				return "", fmt.Errorf("skill_name is required")
			}

			var removed int
			var kind string
			var err error

			switch strings.ToLower(strings.TrimSpace(params.Target)) {
			case "task":
				kind = "task"
				removed, err = runtime.RemoveTask(skillName)
			case "cron":
				kind = "cron"
				removed, err = runtime.RemoveCron(skillName)
			default:
				return "", fmt.Errorf("target must be 'task' or 'cron' (got %q)", params.Target)
			}
			if err != nil {
				return "", err
			}
			if removed == 0 {
				return fmt.Sprintf("no %s found for skill %q", kind, skillName), nil
			}
			if err := skill.TrashSchedule(ctx, skillName); err != nil {
				return "", fmt.Errorf("TrashScheduleSkill: %w", err)
			}
			return fmt.Sprintf("removed %d %s(s) for skill %q and moved skill to .Trash", removed, kind, skillName), nil
		},
	})
}

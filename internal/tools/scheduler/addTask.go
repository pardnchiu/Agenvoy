package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registAddTask() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_task",
		Description: "Low-level time-binding for an existing scheduler skill. DO NOT call directly when the user asks for a new reminder/scheduled task — use the scheduler-skill-creator skill instead (it creates the skill body then calls this tool). Use this tool directly ONLY when the skill_name already exists under ~/.config/agenvoy/skills/scheduler/<name>/ and the user is rebinding/changing the time.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"time": map[string]any{
					"type":        "string",
					"description": "Fire time: '+5m' / '+1h30m' (relative), '15:04' (today clock), '2006-01-02 15:04' (local datetime), or RFC3339.",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill short name (e.g. 'meeting-reminder'). No 'scheduler-' prefix. Must already exist under ~/.config/agenvoy/skills/scheduler/<name>/.",
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

			at, err := parseTime(params.Time)
			if err != nil {
				return "", err
			}
			if !at.After(time.Now()) {
				return "", fmt.Errorf("already gone: %s", at.Local().Format("2006-01-02 15:04:05"))
			}

			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}
			if !filesystem.ScheduleSkillExists(skill) {
				return "", fmt.Errorf("skill %q not found under %s. If the user is requesting a new reminder/scheduled task, you should be running the scheduler-skill-creator skill (which handles create + bind in one flow), not calling add_task directly. If you have just created the skill, call add_task again with the same arguments now to complete the binding", skill, filesystem.ScheduleSkillPath(skill))
			}

			entry := runtime.TaskEntry{
				At:        at.UTC(),
				SessionID: strings.TrimSpace(e.SessionID),
				Skill:     skill,
			}
			if err := runtime.AppendTask(entry); err != nil {
				return "", err
			}

			return fmt.Sprintf("task scheduled: %s fires at %s",
				skill, at.Local().Format("2006-01-02 15:04:05")), nil
		},
	})
}

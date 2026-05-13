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
		Description: "Internal time-binding called by the scheduler-skill-creator skill flow. LLM must NOT call directly — every scheduler skill uses a hash-suffixed name that only scheduler-skill-creator generates, so any hand-made skill_name will fail.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"time": map[string]any{
					"type":        "string",
					"description": "Fire time: '+5m' / '+1h30m' (relative), '15:04' (today clock), '2006-01-02 15:04' (local datetime), or RFC3339.",
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
				return "", fmt.Errorf("skill %q not found under %s. add_task is an internal binding called by the scheduler-skill-creator skill flow. Run scheduler-skill-creator skill which generates a hashed skill name and binds time in one flow. Do not call add_task with a hand-made name", skill, filesystem.ScheduleSkillPath(skill))
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

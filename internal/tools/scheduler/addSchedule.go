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
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

func registAddSchedule() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "add_schedule",
		Description: "Internal schedule-binding called by the scheduler-skill-creator skill flow. LLM must NOT call directly — every scheduler skill uses a hash-suffixed name that only scheduler-skill-creator generates, so any hand-made skill_name will fail.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"enum":        []string{"task", "cron"},
					"description": "Schedule type: 'task' for one-shot fire time, 'cron' for recurring 5-field cron expression.",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "For task: '+5m' / '+1h30m' (relative), '15:04' (today clock), '2006-01-02 15:04' (local datetime), or RFC3339. For cron: standard 5-field expression '{min} {hour} {dom} {mon} {dow}'.",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Hashed scheduler skill name '<short>-<hash8>' produced by scheduler-skill-creator (no 'scheduler-' prefix). Never hand-craft this value.",
				},
			},
			"required": []string{"target", "time", "skill_name"},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Target    string `json:"target"`
				Time      string `json:"time"`
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}
			if !go_pkg_filesystem_reader.Exists(filesystem.ScheduleSkillPath(skill)) {
				return "", fmt.Errorf("skill %q not found under %s. add_schedule is an internal binding called by the scheduler-skill-creator skill flow. Run scheduler-skill-creator skill which generates a hashed skill name and binds time in one flow. Do not call add_schedule with a hand-made name", skill, filesystem.ScheduleSkillPath(skill))
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

			case "cron":
				expression := strings.TrimSpace(params.Time)
				if len(strings.Fields(expression)) != 5 {
					return "", fmt.Errorf("expression must be 5 fields '{min} {hour} {dom} {mon} {dow}' (got %q)", expression)
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

			default:
				return "", fmt.Errorf("target must be 'task' or 'cron' (got %q)", params.Target)
			}
		},
	})
}

package scheduler

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registSchedules() {
	toolregister.Regist(toolregister.Def{
		Name:        "schedules",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  false,
		Description: `This session's scheduled runs, each bound to a scheduler skill: list what is queued, move one to a new time (patch), or cancel one (remove).
Use for "what's scheduled", "move my reminder to 9am", "cancel that schedule", "is that recurring job still on".
Creating a schedule → the scheduler-skill-creator skill, never mode=write. A dry-run request → find the name with mode=list, read its SKILL.md and run the steps now; never answer "run /sched-X in the TUI".`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"list", "patch", "remove", "write"},
					"description": "list: what this session has queued. patch: move an entry to a new time. remove: cancel it and move its skill to .Trash. write: the internal binding step of scheduler-skill-creator; its skill_name carries a hash only that flow produces, so a hand-made one always fails.",
					"default":     "list",
				},
				"target": map[string]any{
					"type":        "string",
					"enum":        []string{"task", "cron", "all"},
					"description": "Which kind of entry. task: one-shot fire time. cron: recurring schedule. all: both, mode=list only and what list uses when omitted. mode=patch / remove / write must pass task or cron explicitly.",
					"default":     "all",
				},
				"skill_name": map[string]any{
					"type":        "string",
					"description": "mode=patch / remove / write, required: the scheduler skill's full name as mode=list shows it, hash suffix included and no 'scheduler-' prefix, e.g. 'meeting-reminder-a3f9b2c1'.",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "mode=patch / write, required. target=task, a future time: '+5m' / '+1h30m' (relative), '15:04' (later today only; no rollover to tomorrow), '2006-01-02 15:04' (local datetime), or RFC3339. target=cron: 5-field '{min} {hour} {dom} {mon} {dow}' e.g. '0 9 * * 1-5', a descriptor (@yearly @annually @monthly @weekly @daily @midnight @hourly), or '@every {duration}' (minimum 30s).",
				},
			},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode      string `json:"mode"`
				Target    string `json:"target"`
				SkillName string `json:"skill_name"`
				Time      string `json:"time"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			mode := strings.ToLower(strings.TrimSpace(params.Mode))
			if mode == "" {
				mode = "list"
			}
			target := strings.ToLower(strings.TrimSpace(params.Target))
			skill := strings.TrimSpace(params.SkillName)

			if mode == "list" {
				if target == "" {
					target = "all"
				}
				if target != "task" && target != "cron" && target != "all" {
					return "", utilsmsg.New(utilsmsg.Invalid, "target", "must be 'task', 'cron', or 'all' (got %q)", target)
				}
				return listSchedule(e, target)
			}

			if skill == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "skill_name", "required when mode=%s", mode)
			}
			if target != "task" && target != "cron" {
				return "", utilsmsg.New(utilsmsg.Invalid, "target", "must be 'task' or 'cron' when mode=%s (got %q)", mode, target)
			}

			switch mode {
			case "write":
				if strings.TrimSpace(params.Time) == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "time", "required when mode=write")
				}
				return writeSchedule(e, target, params.Time, skill)
			case "patch":
				if strings.TrimSpace(params.Time) == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "time", "required when mode=patch")
				}
				return patchSchedule(target, params.Time, skill)
			case "remove":
				return removeSchedule(ctx, e, target, skill)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown mode %q; available: list, patch, remove, write", mode)
		},
	})
}

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListSchedule() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_schedule",
		Description: "List scheduled tasks and/or crons in current session. For test/dry-run requests: find skill name here, then read_file SKILL.md and execute directly — never reply with 'run /sched-X in TUI'.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target": map[string]any{
					"type":        "string",
					"enum":        []string{"task", "cron", "all"},
					"description": "Which schedule type to list. Default 'all'.",
					"default":     "all",
				},
			},
		},
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Target string `json:"target"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			sid := ""
			if e != nil {
				sid = strings.TrimSpace(e.SessionID)
			}

			target := strings.ToLower(strings.TrimSpace(params.Target))
			if target == "" {
				target = "all"
			}

			type result struct {
				Tasks []runtime.TaskEntry `json:"tasks,omitempty"`
				Crons []runtime.CronEntry `json:"crons,omitempty"`
			}
			var r result

			if target == "task" || target == "all" {
				tasks, err := runtime.LoadTasks()
				if err != nil {
					return "", fmt.Errorf("LoadTasks: %w", err)
				}
				for _, t := range tasks {
					if strings.TrimSpace(t.SessionID) == sid {
						r.Tasks = append(r.Tasks, t)
					}
				}
			}

			if target == "cron" || target == "all" {
				crons, err := runtime.LoadCrons()
				if err != nil {
					return "", fmt.Errorf("LoadCrons: %w", err)
				}
				for _, c := range crons {
					if strings.TrimSpace(c.SessionID) == sid {
						r.Crons = append(r.Crons, c)
					}
				}
			}

			if target != "task" && target != "cron" && target != "all" {
				return "", fmt.Errorf("target must be 'task', 'cron', or 'all' (got %q)", target)
			}

			if len(r.Tasks) == 0 && len(r.Crons) == 0 {
				return "{}", nil
			}

			raw, err := json.Marshal(r)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

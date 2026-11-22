package cron

import (
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func listSchedule(e *tooltypes.Executor, target string) (string, error) {
	sid := ""
	if e != nil {
		sid = strings.TrimSpace(e.SessionID)
	}

	type result struct {
		Tasks []runtime.TaskEntry `json:"tasks,omitempty"`
		Crons []runtime.CronEntry `json:"crons,omitempty"`
	}
	var r result

	if target == "task" || target == "all" {
		tasks, err := runtime.LoadTasks()
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "runtime.LoadTasks", "", err)
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
			return "", utilsmsg.Wrap(utilsmsg.Internal, "runtime.LoadCrons", "", err)
		}
		for _, c := range crons {
			if strings.TrimSpace(c.SessionID) == sid {
				r.Crons = append(r.Crons, c)
			}
		}
	}

	if len(r.Tasks) == 0 && len(r.Crons) == 0 {
		return "{}", nil
	}

	raw, err := json.Marshal(r)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}

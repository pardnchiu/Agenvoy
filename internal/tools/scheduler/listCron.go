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

func registListCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_cron",
		Description: "List cron jobs scheduled in the current session from crons.json. No parameters. Returns JSON array of {expression, session_id, skill} filtered to the caller's session_id; empty array when nothing is scheduled in this session.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Handler: func(_ context.Context, e *toolTypes.Executor, _ json.RawMessage) (string, error) {
			crons, err := runtime.LoadCrons()
			if err != nil {
				return "", fmt.Errorf("LoadCrons: %w", err)
			}
			sid := ""
			if e != nil {
				sid = strings.TrimSpace(e.SessionID)
			}
			filtered := make([]runtime.CronEntry, 0, len(crons))
			for _, c := range crons {
				if strings.TrimSpace(c.SessionID) != sid {
					continue
				}
				filtered = append(filtered, c)
			}
			if len(filtered) == 0 {
				return "[]", nil
			}
			raw, err := json.Marshal(filtered)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

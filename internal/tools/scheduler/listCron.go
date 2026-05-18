package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_cron",
		Description: "List currently scheduled cron jobs from crons.json. No parameters. Returns JSON array of {expression, session_id, skill}; empty array when nothing is scheduled.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Concurrent:  true,
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			crons, err := runtime.LoadCrons()
			if err != nil {
				return "", fmt.Errorf("LoadCrons: %w", err)
			}
			if len(crons) == 0 {
				return "[]", nil
			}
			buf, err := json.Marshal(crons)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(buf), nil
		},
	})
}

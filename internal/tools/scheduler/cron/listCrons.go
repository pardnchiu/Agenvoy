package cronTools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registListCrons() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "list_crons",
		ReadOnly: true,
		Description: "List all active recurring cron tasks.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			results, err := crons.List(scheduler.Get())
			if err != nil {
				return "", err
			}

			if len(results) == 0 {
				return "please add cron first", nil
			}
			return strings.Join(results, ","), nil
		},
	})
}

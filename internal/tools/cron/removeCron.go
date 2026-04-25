package cronTools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/scheduler/crons"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveCron() {
	toolRegister.Regist(toolRegister.Def{
		Name: "remove_cron",
		Description: `
Remove a recurring cron task by ID.
Call only when the user explicitly asks to delete a schedule; never invoke during a creation flow.
If no ID is given, call list_crons first; auto-remove only when exactly one task exists, otherwise return the list and wait for confirmation.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Cron task ID returned by list_crons.",
				},
			},
			"required": []string{
				"id",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			id := strings.TrimSpace(params.ID)
			if id == "" {
				return "", fmt.Errorf("id is required")
			}

			if err := crons.Delete(scheduler.Get(), id); err != nil {
				return "", err
			}
			return fmt.Sprintf("removed cron: %s", id), nil
		},
	})
}

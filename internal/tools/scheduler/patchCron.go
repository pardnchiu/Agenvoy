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

func registPatchCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "patch_cron",
		Description: "Reschedule a recurring cron by skill name. Updates the cron expression of the existing entry without changing the bound skill.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill full name (e.g. 'daily-hn-digest-a3f9b2c1') used to locate the cron entry.",
				},
				"time": map[string]any{
					"type":        "string",
					"description": "New 5-field cron expression '{min} {hour} {dom} {mon} {dow}' (e.g. '*/5 * * * *', '0 9 * * *').",
				},
			},
			"required": []string{"skill_name", "time"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				SkillName string `json:"skill_name"`
				Time      string `json:"time"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}

			expression := strings.TrimSpace(params.Time)
			if len(strings.Fields(expression)) != 5 {
				return "", fmt.Errorf("expression must be 5 fields '{min} {hour} {dom} {mon} {dow}' (got %q)", expression)
			}

			patched, err := runtime.PatchCron(skill, expression)
			if err != nil {
				return "", err
			}
			if patched == 0 {
				return fmt.Sprintf("no cron found for skill %q", skill), nil
			}
			return fmt.Sprintf("patched %d cron(s) for skill %q; new expression: %s",
				patched, skill, expression), nil
		},
	})
}

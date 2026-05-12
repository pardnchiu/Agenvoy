package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRemoveCron() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "remove_cron",
		Description: "Cancel a scheduled recurring cron by its skill name (full name including hash suffix).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill_name": map[string]any{
					"type":        "string",
					"description": "Scheduler skill full name (e.g. 'daily-hn-digest-a3f9b2c1').",
				},
			},
			"required": []string{"skill_name"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				SkillName string `json:"skill_name"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			skill := strings.TrimSpace(params.SkillName)
			if skill == "" {
				return "", fmt.Errorf("skill_name is required")
			}
			removed, err := runtime.RemoveCron(skill)
			if err != nil {
				return "", err
			}
			if removed == 0 {
				return fmt.Sprintf("no cron found for skill %q", skill), nil
			}
			if err := filesystem.TrashScheduleSkill(skill); err != nil {
				return "", fmt.Errorf("TrashScheduleSkill: %w", err)
			}
			return fmt.Sprintf("removed %d cron(s) for skill %q and moved skill to .Trash", removed, skill), nil
		},
	})
}

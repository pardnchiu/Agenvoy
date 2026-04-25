package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registInvokeSubagent() {
	models := []string{}
	for _, m := range exec.GetAgent() {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}

	toolRegister.Regist(toolRegister.Def{
		Name:       "invoke_subagent",
		ReadOnly:   true,
		Concurrent: true,
		Description: "Run a subtask in an isolated internal subagent session and return its final text.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Self-contained task description for the subagent.",
				},
				"model": map[string]any{
					"type":        "string",
					"description": "Worker model name. Leave blank for planner auto-select.",
					"default":     "",
					"enum":        models,
				},
				"system_prompt": map[string]any{
					"type":        "string",
					"description": "Extra role or constraints appended to the subagent's system prompt.",
					"default":     "",
				},
				"exclude_tools": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Tool names to exclude. invoke_subagent is always force-excluded.",
					"default":     []string{},
				},
			},
			"required": []string{
				"task",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Task         string   `json:"task"`
				Model        string   `json:"model,omitempty"`
				SystemPrompt string   `json:"system_prompt,omitempty"`
				ExcludeTools []string `json:"exclude_tools,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			task := strings.TrimSpace(params.Task)
			if task == "" {
				return "", fmt.Errorf("task is required")
			}

			model := strings.TrimSpace(params.Model)
			if model != "" && !slices.Contains(models, model) {
				slog.Warn("invalid model, fallback to auto-select")
				model = ""
			}

			systemPrompt := strings.TrimSpace(params.SystemPrompt)

			excludeTools := params.ExcludeTools
			if excludeTools == nil {
				excludeTools = []string{}
			}

			return exec.ExecWithSubagent(ctx, task, model, systemPrompt, excludeTools)
		},
	})
}

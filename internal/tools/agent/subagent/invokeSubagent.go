package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
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
		Name:        "invoke_subagent",
		ReadOnly:    true,
		Concurrent:  true,
		Description: "Run a subtask in an internal subagent session and return its final text.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Self-contained task description for the subagent.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Friendly identifier matching bot.md frontmatter `name` of an existing cli- session. Resolves to its session_id; takes precedence over session_id when both are set.",
					"default":     "",
				},
				"session_id": map[string]any{
					"type":        "string",
					"description": "Persistent session id to thread multi-turn subagent calls (e.g. 'researcher', 'planner-2'). Blank uses an ephemeral temp-sub session. Ignored when name resolves successfully.",
					"default":     "",
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
					"description": "Extra tool names to exclude on top of the always-excluded set (invoke_subagent, invoke_external_agent, cross_review_with_external_agents, review_result, ask_user). The default set cannot be overridden.",
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
				Name         string   `json:"name,omitempty"`
				SessionID    string   `json:"session_id,omitempty"`
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

			sessionID := strings.TrimSpace(params.SessionID)
			if name := strings.TrimSpace(params.Name); name != "" {
				resolved := sessionManager.GetSessionIDByName(name)
				if resolved == "" {
					return "", fmt.Errorf("no cli- session has bot.md name = %q", name)
				}
				sessionID = resolved
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

			return exec.ExecWithSubagent(ctx, task, sessionID, model, systemPrompt, excludeTools)
		},
	})
}

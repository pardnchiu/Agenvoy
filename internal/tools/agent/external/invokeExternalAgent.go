package externalAgent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registInvokeExternalAgent() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "invoke_external_agent",
		ReadOnly: true,
		Description: `Invoke one external CLI agent (codex / copilot / claude / gemini) for an independent second opinion.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"provider": map[string]any{
					"type":        "string",
					"description": "External agent to invoke.",
					"enum":        []string{"codex", "copilot", "claude", "gemini"},
				},
				"task": map[string]any{
					"type":        "string",
					"description": "Self-contained task description with full context and required output format.",
				},
				"readonly": map[string]any{
					"type":        "boolean",
					"description": "Read-only mode. Defaults to true; set false only when the agent must write files.",
					"default":     true,
				},
			},
			"required": []string{
				"provider",
				"task",
			},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Provider string `json:"provider"`
				Task     string `json:"task"`
				ReadOnly *bool  `json:"readonly,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			readOnly := true
			if params.ReadOnly != nil {
				readOnly = *params.ReadOnly
			}

			if !slices.Contains(external.Agents(), params.Provider) {
				return fmt.Sprintf(
					"please enable in env first, supported agents: %s",
					strings.Join(external.Agents(), ","),
				), nil
			}

			if err := external.Check(params.Provider); err != nil {
				return fmt.Sprintf("failed to check%s: %s", params.Provider, err.Error()), nil
			}

			out, err := external.Call(ctx, params.Provider, params.Task, readOnly)
			if err != nil {
				return fmt.Sprintf("failed to run %s: %s", params.Provider, err.Error()), nil
			}

			return fmt.Sprintf("output from %s:\n%s", params.Provider, out), nil
		},
	})
}

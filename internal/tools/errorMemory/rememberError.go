package errorMemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem/errorMemory"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registRememberError() {
	toolRegister.Regist(toolRegister.Def{
		Name:     "remember_error",
		ReadOnly: true,
		Description: `
Persist a tool-error record for future retrieval via search_tool_errors.
Call after resolving or abandoning the error.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{
					"type":        "string",
					"description": "Tool that produced the error (e.g. 'fetch_page').",
				},
				"keywords": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Lookup keywords — tool name, error type, parameter traits. Be specific.",
				},
				"symptom": map[string]any{
					"type":        "string",
					"description": "Observed behavior — what the tool returned or failed on.",
				},
				"cause": map[string]any{
					"type":        "string",
					"description": "Root cause once confirmed.",
					"default":     "",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Action taken (e.g. 'retried with English keyword', 'fell back to search_web').",
				},
				"outcome": map[string]any{
					"type":        "string",
					"enum":        []string{"resolved", "failed", "abandoned"},
					"description": "resolved = fix worked; failed = strategy confirmed non-working; abandoned = 3+ approaches tried.",
				},
			},
			"required": []string{
				"tool_name",
				"keywords",
				"symptom",
				"action",
				"outcome",
			},
		},
		Handler: func(_ context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ToolName string   `json:"tool_name"`
				Keywords []string `json:"keywords"`
				Symptom  string   `json:"symptom"`
				Cause    string   `json:"cause"`
				Action   string   `json:"action"`
				Outcome  string   `json:"outcome"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}

			toolName := strings.TrimSpace(params.ToolName)
			if toolName == "" {
				return "", fmt.Errorf("tool_name is required")
			}

			keywords := params.Keywords
			if len(keywords) == 0 {
				return "", fmt.Errorf("keywords is required")
			}

			symptom := strings.TrimSpace(params.Symptom)
			if symptom == "" {
				return "", fmt.Errorf("symptom is required")
			}

			cause := strings.TrimSpace(params.Cause)

			action := strings.TrimSpace(params.Action)
			if action == "" {
				return "", fmt.Errorf("action is required")
			}

			outcome := strings.TrimSpace(params.Outcome)
			if outcome == "" {
				return "", fmt.Errorf("outcome is required")
			}
			return errorMemory.Save(e.SessionID, errorMemory.Record{
				ToolName: toolName,
				Keywords: keywords,
				Symptom:  symptom,
				Cause:    cause,
				Action:   action,
				Outcome:  outcome,
			})
		},
	})
}

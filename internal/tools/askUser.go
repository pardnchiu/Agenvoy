package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type askQuestion struct {
	Question    string   `json:"question"`
	Detail      string   `json:"detail,omitempty"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

func registAskUser() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "ask_user",
		AlwaysAllow: true,
		AlwaysLoad:  true,
		Description: "Ask the user one or more questions and return their answers.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"questions": map[string]any{
					"type":        "array",
					"description": "Questions to ask in order. Each item is either a free-text prompt (no options) or a choice prompt (with options).",
					"minItems":    1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"question": map[string]any{
								"type":        "string",
								"description": "Short prompt shown as the popup title (the actual question, e.g. 'Confirm?').",
							},
							"detail": map[string]any{
								"type":        "string",
								"description": "Optional multi-line context / details rendered as popup subtitle in hint style above the input. Use this for the supporting info (list of detected items, current values, etc.) — keep `question` short.",
							},
							"options": map[string]any{
								"type":        "array",
								"description": "Fixed choices. Omit or empty for free-text input.",
								"items":       map[string]any{"type": "string"},
							},
							"multi_select": map[string]any{
								"type":        "boolean",
								"description": "When options is non-empty: true = multi-select (comma-separated indices), false = single-select (arrow keys). Ignored for free-text.",
							},
							"secret": map[string]any{
								"type":        "boolean",
								"description": "Free-text only: true masks input and excludes the answer from logs. Ignored when options is set.",
							},
						},
						"required": []string{"question"},
					},
				},
			},
			"required": []string{"questions"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Questions []askQuestion `json:"questions"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			if len(params.Questions) == 0 {
				return "", fmt.Errorf("ask_user requires at least one question in 'questions'")
			}
			if !runtime.HasListener(e.SessionID) {
				return "", fmt.Errorf("ask_user requires an interactive channel (TUI / Telegram / Discord); current session %q has no listener", e.SessionID)
			}
			questions := make([]runtime.Question, 0, len(params.Questions))
			for i, q := range params.Questions {
				q.Question = strings.TrimSpace(q.Question)
				if q.Question == "" {
					return "", fmt.Errorf("question #%d is empty", i+1)
				}
				questions = append(questions, runtime.Question{
					Question:    q.Question,
					Detail:      strings.TrimSpace(q.Detail),
					Options:     q.Options,
					MultiSelect: q.MultiSelect,
					Secret:      q.Secret,
				})
			}
			reply, err := runtime.Ask(ctx, runtime.Request{
				Kind:      runtime.KindAskUser,
				SessionID: e.SessionID,
				ToolName:  "ask_user",
				AskUser:   &runtime.UserPayload{Questions: questions},
			})
			if err != nil {
				return "", fmt.Errorf("pending.Ask: %w", err)
			}
			if reply.Error != nil {
				return "", reply.Error
			}
			out, err := json.Marshal(map[string]any{"answers": reply.Answers})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

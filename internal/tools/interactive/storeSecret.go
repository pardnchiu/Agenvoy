package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/kuradb"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

func registStoreSecret() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "store_secret",
		AlwaysLoad:  true,
		Description: "Prompt the user for a secret value with masked input and persist it to the system keychain under the given key.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Keychain entry name (e.g. OPENAI_API_KEY).",
				},
				"question": map[string]any{
					"type":        "string",
					"description": "Optional question text shown to the user. Defaults to a generic prompt referencing the key.",
				},
			},
			"required": []string{"key"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Key      string `json:"key"`
				Question string `json:"question"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json Unmarshal: %w", err)
			}

			key := strings.TrimSpace(params.Key)
			if key == "" {
				return "", fmt.Errorf("key is required")
			}

			question := strings.TrimSpace(params.Question)
			if question == "" {
				question = fmt.Sprintf("%s is required", key)
			}

			value, err := SecretPrompt(ctx, e.SessionID, question)
			if err != nil {
				return "", fmt.Errorf("SecretPrompt: %w", err)
			}
			if value == "" {
				return "", fmt.Errorf("%s is empty", key)
			}

			if err := keychain.Set(key, value); err != nil {
				return "", fmt.Errorf("keychain Set: %w", err)
			}
			if key == "OPENAI_API_KEY" {
				if err := kuradb.SyncOpenAIKey(value); err != nil {
					return "", fmt.Errorf("kuradb SyncOpenAIKey: %w", err)
				}
			}

			if err := config.SaveKey(key); err != nil {
				return "", fmt.Errorf("session SaveKey: %w", err)
			}

			raw, err := json.Marshal(map[string]any{"ok": true, "key": key})
			if err != nil {
				return "", fmt.Errorf("json Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

func SecretPrompt(ctx context.Context, sessionID, question string) (string, error) {
	if !runtime.HasListener(sessionID) {
		return "", fmt.Errorf("store_secret requires an interactive channel (TUI / Telegram / Discord)")
	}

	reply, err := runtime.Ask(ctx, runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		ToolName:  "store_secret",
		AskUser: &runtime.UserPayload{
			Questions: []runtime.Question{{Question: question, Secret: true}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("runtime Ask: %w", err)
	}
	if reply.Error != nil {
		return "", fmt.Errorf("runtime Ask: %s", reply.Error)
	}
	if len(reply.Answers) == 0 {
		return "", fmt.Errorf("no answers")
	}

	str, ok := reply.Answers[0].(string)
	if !ok {
		return "", fmt.Errorf("non-string answer: %T", reply.Answers[0])
	}
	return str, nil
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
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
				"prompt": map[string]any{
					"type":        "string",
					"description": "Optional question text shown to the user. Defaults to a generic prompt referencing the key.",
				},
			},
			"required": []string{"key"},
		},
		Handler: func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Key    string `json:"key"`
				Prompt string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			params.Key = strings.TrimSpace(params.Key)
			if params.Key == "" {
				return "", fmt.Errorf("key is required")
			}

			question := strings.TrimSpace(params.Prompt)
			if question == "" {
				question = fmt.Sprintf("請輸入 %s 的值", params.Key)
			}

			value, err := readSecretValue(ctx, e.SessionID, question)
			if err != nil {
				return "", err
			}
			if value == "" {
				return "", fmt.Errorf("user provided empty value")
			}

			if err := keychain.Set(params.Key, value); err != nil {
				return "", fmt.Errorf("keychain.Set: %w", err)
			}
			if err := sessionManager.SaveKey(params.Key); err != nil {
				return "", fmt.Errorf("sessionManager.SaveKey: %w", err)
			}

			out, err := json.Marshal(map[string]any{"ok": true, "key": params.Key})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

func readSecretValue(ctx context.Context, sessionID, question string) (string, error) {
	if !runtime.HasListener(sessionID) {
		return "", fmt.Errorf("store_secret requires an interactive channel (TUI / Telegram / Discord); current session %q has no listener", sessionID)
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
		return "", fmt.Errorf("pending.Ask: %w", err)
	}
	if reply.Error != nil {
		return "", reply.Error
	}
	if len(reply.Answers) == 0 {
		return "", fmt.Errorf("pending.Ask returned no answers")
	}
	s, ok := reply.Answers[0].(string)
	if !ok {
		return "", fmt.Errorf("pending.Ask returned non-string answer: %T", reply.Answers[0])
	}
	return s, nil
}

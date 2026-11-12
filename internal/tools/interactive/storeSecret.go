package interactive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registStoreSecret() {
	toolregister.Regist(toolregister.Def{
		Name:        "store_secret",
		SystemUse:   true,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: `Asks the user for a secret with masked input and stores it in the OS keychain; returns only the key name, never the value.
Use for "set my OpenAI key", "update my API token", and on an auth failure (missing key, 401, 403, expired token): take the key name from the error, call this, then re-invoke the tool that failed. Other questions to the user → ask_user.
Stored keys already live in the OS keychain under service ` + "`agenvoy`" + `, account = the key name (OPENAI_API_KEY, POLYGON_API_KEY, ...): macOS ` + "`security find-generic-password -s agenvoy -a <KEY> -w`" + `, Linux ` + "`secret-tool lookup service agenvoy account <KEY>`" + ` then ` + "`~/.config/agenvoy/.secrets`" + `, and only after those the environment variable of that name. Nothing is exported into the process environment, so an empty ` + "`printenv`" + ` is no reason to ask the user or call the task blocked; read from there when a call needs credentials, and use this tool only for a key that is not there yet.
Credentials travel only through this tool's masked input; keep the value out of messages, tool arguments and files. At most two rounds per tool per turn.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Keychain entry name, usually the environment-variable name from the error, e.g. 'OPENAI_API_KEY', 'POLYGON_API_KEY'.",
				},
				"question": map[string]any{
					"type":        "string",
					"description": "Prompt shown to the user, e.g. 'Paste your Polygon API key (polygon.io/dashboard)'; omitted → '<key> is required'.",
				},
			},
			"required": []string{"key"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Key      string `json:"key"`
				Question string `json:"question"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}

			key := strings.TrimSpace(params.Key)
			if key == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "key", "required")
			}

			question := strings.TrimSpace(params.Question)
			if question == "" {
				question = fmt.Sprintf("%s is required", key)
			}

			value, err := SecretPrompt(ctx, e.SessionID, question)
			if err != nil {
				return "", utilsmsg.Trace("SecretPrompt", "", err)
			}
			if value == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, key, "empty")
			}

			if err := keychain.Set(key, value); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "keychain.Set", key, err)
			}

			if err := config.SaveKey(key); err != nil {
				return "", utilsmsg.Trace("config.SaveKey", key, err)
			}

			raw, err := json.Marshal(map[string]any{"ok": true, "key": key})
			if err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
			}
			return string(raw), nil
		},
	})
}

func SecretPrompt(ctx context.Context, sessionID, question string) (string, error) {
	origin := originFor(ctx, sessionID)
	if agenttypes.AutoModeFrom(ctx) != agenttypes.AutoModeOff || !runtime.HasListener(ctx, origin) {
		return "", utilsmsg.New(utilsmsg.Unavailable, "store_secret", "requires an interactive channel (TUI / Telegram / Discord)")
	}

	reply, err := runtime.Ask(ctx, runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		Origin:    origin,
		DeliverTo: deliverFor(ctx, sessionID),
		ToolName:  "store_secret",
		AskUser: &runtime.UserPayload{
			Questions: []runtime.Question{{Question: question, Secret: true}},
		},
	})
	if err != nil {
		return "", utilsmsg.Trace("runtime.Ask", "", err)
	}
	if reply.Error != nil {
		return "", utilsmsg.Trace("runtime.Ask", "", reply.Error)
	}
	if len(reply.Answers) == 0 {
		return "", utilsmsg.New(utilsmsg.Internal, "runtime.Ask", "no answers")
	}

	str, ok := reply.Answers[0].(string)
	if !ok {
		return "", utilsmsg.New(utilsmsg.Internal, "runtime.Ask", "non-string answer: %T", reply.Answers[0])
	}
	return str, nil
}

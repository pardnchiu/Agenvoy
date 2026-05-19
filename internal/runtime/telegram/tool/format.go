package tool

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registTelegramFormat() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "telegram_format",
		AlwaysLoad:  true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default]
Return the complete Telegram HTML formatting reference (allowed tags, escape rules, forbidden markdown, file/voice markers, concrete rewrite table).
Call this BEFORE composing any content that will be delivered to Telegram:
- you are in a tg- session (foreground reply, scheduling ack, skill result, push)
- you are about to call send_to_telegram_chat from any session
- you are authoring a script whose stdout will be forwarded to a Telegram chat`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return configs.TelegramFormat, nil
		},
	})
}

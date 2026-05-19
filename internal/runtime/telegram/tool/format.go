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
		Description: `[system-default] Return the complete Telegram HTML formatting reference (allowed tags, escape rules, forbidden markdown, file/voice markers).`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return configs.TelegramFormat, nil
		},
	})
}

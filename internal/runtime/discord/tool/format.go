package tool

import (
	"context"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registDiscordFormat() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "discord_format",
		AlwaysLoad:  true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default] Return the complete Discord markdown formatting reference (allowed markdown, special tokens, image formats, file/voice markers).`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return configs.DiscordFormat, nil
		},
	})
}

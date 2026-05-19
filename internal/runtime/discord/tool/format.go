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
		Description: `[system-default]
Return the complete Discord markdown formatting reference (inline/block markdown, code-block languages, special tokens, image formats, file/voice markers, limits).
Call this BEFORE composing any content that will be delivered to Discord:
- you are in a dc- session (foreground reply, scheduling ack, skill result, push)
- you are about to call send_to_discord_channel from any session
- you are authoring a script whose stdout will be forwarded to a Discord channel`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			return configs.DiscordFormat, nil
		},
	})
}

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func registChatbotFormat() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "format_chatbot",
		AlwaysLoad:  true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default] Return the complete formatting reference for the specified chat platform (Telegram HTML or Discord markdown).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"platform": platformParam(),
			},
			"required": []string{"platform"},
		},
		Handler: func(_ context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Platform string `json:"platform"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			platform, err := parsePlatform(strings.TrimSpace(params.Platform))
			if err != nil {
				return "", err
			}
			switch platform {
			case platformTelegram:
				return configs.TelegramFormat, nil
			case platformDiscord:
				return configs.DiscordFormat, nil
			}
			return "", fmt.Errorf("unreachable platform %q", platform)
		},
	})
}

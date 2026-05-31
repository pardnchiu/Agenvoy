package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func registSendToDiscordChannel() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "send_to_discord_channel",
		Description: `[system-default] Send a markdown-formatted message to an authorized Discord channel by channel_id.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"channel_id": map[string]any{
					"type":        "string",
					"description": "Discord channel id (from list_discord_channel).",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Discord-markdown message body.",
				},
			},
			"required": []string{"channel_id", "message"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ChannelID string `json:"channel_id"`
				Message   string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			channelID := strings.TrimSpace(params.ChannelID)
			message := strings.TrimSpace(params.Message)
			if channelID == "" {
				return "", fmt.Errorf("channel_id is required")
			}
			if message == "" {
				return "", fmt.Errorf("message is required")
			}

			if !utils.IsAuthorized(filesystem.DiscordAuthPath, channelID) {
				return "", fmt.Errorf("channel_id %q is not in %s; call list_discord_channel for authorized targets", channelID, filesystem.DiscordAuthPath)
			}

			token := strings.TrimSpace(keychain.Get(discord.Key))
			if token == "" {
				return "", fmt.Errorf("keychain entry %q missing; enable Discord via TUI /discord", discord.Key)
			}

			client, err := go_bot_discord.New(token)
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-bot/discord New: %w", err)
			}

			msg, err := client.Send(ctx, channelID, "", message)
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-bot/discord Bot.Send: %w", err)
			}

			raw, err := json.Marshal(map[string]any{
				"ok":         true,
				"channel_id": channelID,
				"message_id": msg.ID,
			})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(raw), nil
		},
	})
}

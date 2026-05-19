package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func registListDiscordChannel() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "list_discord_channel",
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `[system-default] List authorized Discord channels (id + name).`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, _ json.RawMessage) (string, error) {
			entries := utils.ListChats(filesystem.DiscordAuthPath)
			if entries == nil {
				entries = []utils.ChatEntry{}
			}
			out, err := json.Marshal(entries)
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

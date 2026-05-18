package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
)

func sendAttachments(ctx context.Context, client *go_bot_discord.Bot, channelID, channelName, replyTo string, paths []string) {
	if client == nil || len(paths) == 0 {
		return
	}

	notifyFailure := func(label, detail, errMsg string) {
		text := fmt.Sprintf("-# ⎿ ⚠️ %s failed", label)
		if detail != "" {
			text = fmt.Sprintf("%s: `%s`", text, detail)
		}
		text = fmt.Sprintf("%s\n-# ⎿ `%s`", text, errMsg)
		if _, err := client.Send(ctx, channelID, replyTo, text); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	for start := 0; start < len(paths); start += 10 {
		end := min(start+10, len(paths))
		batch := paths[start:end]
		if _, err := client.SendFiles(ctx, channelID, replyTo, batch); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.SendFiles",
				slog.String("channel", channelName),
				slog.Int("count", len(batch)),
				slog.String("error", err.Error()))
			notifyFailure("SendFiles", strings.Join(batch, ", "), err.Error())
		}
	}
}

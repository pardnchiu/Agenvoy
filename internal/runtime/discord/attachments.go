package discord

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
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

func saveAttachments(ctx context.Context, b *Bot, in go_bot_discord.Input) []string {
	if b == nil || b.client == nil || len(in.Attachments) == 0 {
		return nil
	}

	dir := filepath.Join(filesystem.AgenvoyDir, "download")
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem CheckDir",
			slog.String("dir", dir),
			slog.String("error", err.Error()))
		return nil
	}

	var paths []string
	for _, att := range in.Attachments {
		if att == nil {
			continue
		}
		path, err := b.client.Save(ctx, att, dir)
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.Save",
				slog.String("channel", channelName(in)),
				slog.String("filename", att.Filename),
				slog.String("error", err.Error()))
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

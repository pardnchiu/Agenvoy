package discord

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	go_bot_discord "github.com/pardnchiu/go-bot/discord"
)

func sendAttachments(ctx context.Context, client *go_bot_discord.Bot, channelID, channelName, replyTo string, paths []string) {
	if client == nil || len(paths) == 0 {
		return
	}

	sendFailure := func(label, detail, errMsg string) {
		str := fmt.Sprintf("-# ⎿ ⚠️ %s failed (background upload)", label)
		if detail != "" {
			str = fmt.Sprintf("%s: `%s`", str, detail)
		}
		str = fmt.Sprintf("%s\n-# ⎿ `%s`", str, errMsg)
		if _, err := client.Send(ctx, channelID, replyTo, str); err != nil {
			slog.Error("github.com/pardnchiu/go-bot/discord Bot.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	for start := 0; start < len(paths); start += 10 {
		end := min(start+10, len(paths))
		batch := paths[start:end]
		if _, err := client.SendFiles(ctx, channelID, replyTo, batch); err != nil {
			slog.Error("github.com/pardnchiu/go-bot/discord Bot.SendFiles",
				slog.String("channel", channelName),
				slog.Int("count", len(batch)),
				slog.String("paths", strings.Join(batch, ", ")),
				slog.String("error", err.Error()))
			sendFailure("SendFiles", strings.Join(batch, ", "), err.Error())
		}
	}
}

func saveAttachments(ctx context.Context, b *Bot, in go_bot_discord.Input) []chatbot.SavedAttachment {
	if b == nil || b.client == nil || len(in.Attachments) == 0 {
		return nil
	}

	dir := filesystem.DownloadDir
	var saved []chatbot.SavedAttachment
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
		saved = append(saved, chatbot.SavedAttachment{Path: path, Transcribe: shouldTranscribeAttachment(att.ContentType, att.Filename)})
	}
	return saved
}

func shouldTranscribeAttachment(contentType, filename string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".ogg", ".oga", ".opus", ".mp3", ".wav", ".m4a", ".flac", ".aac", ".aiff", ".mp4", ".mov", ".webm", ".mpg", ".mpeg", ".3gp":
		return true
	default:
		return false
	}
}

func hasVoiceAttachment(in go_bot_discord.Input) bool {
	for _, att := range in.Attachments {
		if att == nil {
			continue
		}
		if shouldTranscribeAttachment(att.ContentType, att.Filename) {
			return true
		}
	}
	return false
}

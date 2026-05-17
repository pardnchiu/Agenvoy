package telegram

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
)

func saveAttachments(ctx context.Context, b *Bot, in go_bot_telegram.Input) []string {
	if b == nil || b.client == nil {
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

	save := func(kind, fileID string) {
		if fileID == "" {
			return
		}
		path, err := b.client.Save(ctx, fileID, dir)
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Save",
				slog.String("kind", kind),
				slog.Int64("chat", in.ChatID),
				slog.String("error", err.Error()))
			return
		}
		slog.Info("attachment saved",
			slog.String("kind", kind),
			slog.Int64("chat", in.ChatID),
			slog.String("path", path))
		paths = append(paths, path)
	}

	if len(in.Photo) > 0 {
		largest := in.Photo[len(in.Photo)-1]
		save("photo", largest.FileID)
	}
	if in.Document != nil {
		save("document", in.Document.FileID)
	}
	if len(in.Photo) == 0 && in.Document == nil && in.Raw != nil && in.Raw.Message != nil {
		m := in.Raw.Message
		switch {
		case m.Voice != nil:
			save("voice", m.Voice.FileID)
		case m.Audio != nil:
			save("audio", m.Audio.FileID)
		case m.Video != nil:
			save("video", m.Video.FileID)
		case m.VideoNote != nil:
			save("video_note", m.VideoNote.FileID)
		}
	}

	return paths
}

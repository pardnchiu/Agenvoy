package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const attachmentSendTimeout = 10 * time.Minute

func sendAttachments(ctx context.Context, chatID int64, chatName string, replyToID int, photoPaths, docPaths []string) {
	if len(photoPaths) == 0 && len(docPaths) == 0 {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("chat", chatName),
			slog.String("key", Key))
		return
	}

	client, err := go_bot_telegram.New(token,
		go_bot_telegram.WithHTTPClient(&http.Client{Timeout: attachmentSendTimeout}))
	if err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/telegram New",
			slog.String("chat", chatName),
			slog.String("error", err.Error()))
		return
	}

	notifyFailure := func(label, detail, errMsg string) {
		body := fmt.Sprintf("<code>%s</code>", html.EscapeString(errMsg))
		if detail != "" {
			body = fmt.Sprintf("<code>%s</code>: %s", html.EscapeString(detail), body)
		}
		text := fmt.Sprintf("⚠️ %s failed\n%s", label, body)
		if _, err := client.Send(ctx, chatID, replyToID, text, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	for start := 0; start < len(photoPaths); start += 10 {
		end := start + 10
		end = min(end, len(photoPaths))
		if _, err := client.SendPhoto(ctx, chatID, photoPaths[start:end]); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.SendPhoto",
				slog.String("chat", chatName),
				slog.Int("count", end-start),
				slog.String("error", err.Error()))
			notifyFailure("SendPhoto", strings.Join(photoPaths[start:end], ", "), err.Error())
		}
	}
	for _, path := range docPaths {
		if _, err := client.SendFile(ctx, chatID, go_bot_telegram.TypeDocument, path); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.SendFile",
				slog.String("chat", chatName),
				slog.String("path", path),
				slog.String("error", err.Error()))
			notifyFailure("SendFile", path, err.Error())
		}
	}
}

func saveAttachments(ctx context.Context, b *Bot, in go_bot_telegram.Input) []string {
	if b == nil || b.client == nil {
		return nil
	}

	var fileIDs []string
	if len(in.Photo) > 0 {
		fileIDs = append(fileIDs, in.Photo[len(in.Photo)-1].FileID)
	}
	if in.Document != nil {
		fileIDs = append(fileIDs, in.Document.FileID)
	}
	if in.Raw != nil && in.Raw.Message != nil {
		m := in.Raw.Message
		if m.Voice != nil {
			fileIDs = append(fileIDs, m.Voice.FileID)
		}
		if m.Audio != nil {
			fileIDs = append(fileIDs, m.Audio.FileID)
		}
		if m.Video != nil {
			fileIDs = append(fileIDs, m.Video.FileID)
		}
		if m.VideoNote != nil {
			fileIDs = append(fileIDs, m.VideoNote.FileID)
		}
	}
	if len(fileIDs) == 0 {
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
	for _, id := range fileIDs {
		path, err := b.client.Save(ctx, id, dir)
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Save",
				slog.String("chat", chatName(in)),
				slog.String("fileID", id),
				slog.String("error", err.Error()))
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

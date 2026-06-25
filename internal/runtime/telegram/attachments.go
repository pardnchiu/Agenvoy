package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const attachmentSendTimeout = 10 * time.Minute

func sendAttachments(ctx context.Context, chatID int64, chatName string, photoPaths, docPaths []string) {
	if len(photoPaths) == 0 && len(docPaths) == 0 {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Error("github.com/pardnchiu/go-pkg/filesystem/keychain Get",
			slog.String("chat", chatName),
			slog.String("key", Key))
		return
	}

	client, err := go_bot_telegram.New(token,
		go_bot_telegram.WithHTTPClient(&http.Client{Timeout: attachmentSendTimeout}))
	if err != nil {
		slog.Error("github.com/pardnchiu/go-bot/telegram New",
			slog.String("chat", chatName),
			slog.String("error", err.Error()))
		return
	}

	notifyFailure := func(label, detail, errMsg string) {
		body := fmt.Sprintf("<code>%s</code>", html.EscapeString(errMsg))
		if detail != "" {
			body = fmt.Sprintf("<code>%s</code>: %s", html.EscapeString(detail), body)
		}
		str := fmt.Sprintf("⚠️ %s failed (background upload)\n%s", label, body)
		if _, err := client.Send(ctx, chatID, 0, str, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Error("github.com/pardnchiu/go-bot/telegram Bot.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	for start := 0; start < len(photoPaths); start += 10 {
		end := start + 10
		end = min(end, len(photoPaths))
		if _, err := client.SendPhoto(ctx, chatID, photoPaths[start:end]); err != nil {
			slog.Error("github.com/pardnchiu/go-bot/telegram Bot.SendPhoto",
				slog.String("chat", chatName),
				slog.Int("count", end-start),
				slog.String("paths", strings.Join(photoPaths[start:end], ", ")),
				slog.String("error", err.Error()))
			notifyFailure("SendPhoto", strings.Join(photoPaths[start:end], ", "), err.Error())
		}
	}
	for _, path := range docPaths {
		if _, err := client.SendFile(ctx, chatID, go_bot_telegram.TypeDocument, path); err != nil {
			slog.Error("github.com/pardnchiu/go-bot/telegram Bot.SendFile",
				slog.String("chat", chatName),
				slog.String("path", path),
				slog.String("error", err.Error()))
			notifyFailure("SendFile", path, err.Error())
		}
	}
}

func saveAttachments(ctx context.Context, b *Bot, in go_bot_telegram.Input) []chatbot.SavedAttachment {
	if b == nil || b.client == nil {
		return nil
	}

	var items []struct {
		fileID     string
		transcribe bool
	}
	if len(in.Photo) > 0 {
		items = append(items, struct {
			fileID     string
			transcribe bool
		}{fileID: in.Photo[len(in.Photo)-1].FileID})
	}
	if in.Document != nil {
		items = append(items, struct {
			fileID     string
			transcribe bool
		}{fileID: in.Document.FileID})
	}
	if in.Raw != nil && in.Raw.Message != nil {
		m := in.Raw.Message
		if m.Voice != nil {
			items = append(items, struct {
				fileID     string
				transcribe bool
			}{fileID: m.Voice.FileID, transcribe: true})
		}
		if m.Audio != nil {
			items = append(items, struct {
				fileID     string
				transcribe bool
			}{fileID: m.Audio.FileID, transcribe: true})
		}
		if m.Video != nil {
			items = append(items, struct {
				fileID     string
				transcribe bool
			}{fileID: m.Video.FileID, transcribe: true})
		}
		if m.VideoNote != nil {
			items = append(items, struct {
				fileID     string
				transcribe bool
			}{fileID: m.VideoNote.FileID, transcribe: true})
		}
	}
	if len(items) == 0 {
		return nil
	}

	dir := filesystem.DownloadDir
	var saved []chatbot.SavedAttachment
	for _, item := range items {
		path, err := b.client.Save(ctx, item.fileID, dir)
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Save",
				slog.String("chat", chatName(in)),
				slog.String("fileID", item.fileID),
				slog.String("error", err.Error()))
			continue
		}
		saved = append(saved, chatbot.SavedAttachment{Path: path, Transcribe: item.transcribe})
	}
	return saved
}

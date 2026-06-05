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

	"github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/stt"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const attachmentSendTimeout = 10 * time.Minute

type savedAttachment struct {
	path       string
	transcribe bool
}

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

func saveAttachments(ctx context.Context, b *Bot, in go_bot_telegram.Input) []savedAttachment {
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

	dir := filepath.Join(filesystem.AgenvoyDir, "download")
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		slog.Warn("github.com/pardnchiu/go-pkg/filesystem CheckDir",
			slog.String("dir", dir),
			slog.String("error", err.Error()))
		return nil
	}

	var saved []savedAttachment
	for _, item := range items {
		path, err := b.client.Save(ctx, item.fileID, dir)
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/telegram Bot.Save",
				slog.String("chat", chatName(in)),
				slog.String("fileID", item.fileID),
				slog.String("error", err.Error()))
			continue
		}
		saved = append(saved, savedAttachment{path: path, transcribe: item.transcribe})
	}
	return saved
}

func transcribeSavedAttachments(ctx context.Context, attachments []savedAttachment) ([]string, []string, error) {
	var transcripts []string
	var paths []string
	for _, attachment := range attachments {
		if attachment.path == "" {
			continue
		}
		if !attachment.transcribe {
			paths = append(paths, attachment.path)
			continue
		}
		text, err := stt.Transcribe(ctx, attachment.path, "")
		if err != nil {
			return nil, nil, fmt.Errorf("transcribe %s: %w", attachment.path, err)
		}
		if text = strings.TrimSpace(text); text != "" {
			transcripts = append(transcripts, text)
		}
	}
	return transcripts, paths, nil
}

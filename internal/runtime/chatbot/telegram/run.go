package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	go_bot_telegram "github.com/pardnchiu/go-bot/core/telegram"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	execchatbot "github.com/pardnchiu/agenvoy/internal/agents/exec/chatbot"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessionhistory "github.com/pardnchiu/agenvoy/internal/session/history"
	sessiontelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	audiotool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func chatName(in go_bot_telegram.Input) string {
	if in.ChatName != "" {
		return in.ChatName
	}
	return in.Username
}

func inputHasAttachment(in go_bot_telegram.Input) bool {
	if len(in.Photo) > 0 || in.Document != nil {
		return true
	}
	if in.Raw != nil && in.Raw.Message != nil {
		m := in.Raw.Message
		return m.Voice != nil || m.Audio != nil || m.Video != nil || m.VideoNote != nil
	}
	return false
}

func inputHasVoice(in go_bot_telegram.Input) bool {
	if in.Raw == nil || in.Raw.Message == nil {
		return false
	}
	m := in.Raw.Message
	return m.Voice != nil || m.Audio != nil || m.Video != nil || m.VideoNote != nil
}

func recordChatter(in go_bot_telegram.Input, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	sessionID, err := sessiontelegram.New(in.ChatID)
	if err != nil {
		slog.Debug("sessiontelegram.New (chatter)",
			slog.Int64("chat", in.ChatID),
			slog.String("error", err.Error()))
		return
	}

	username := in.Username
	if username == "" {
		username = "unknown"
	}
	if err := sessionhistory.Append(sessionID, []sessionhistory.Record{{
		Role:    "user",
		Content: content,
		SendAt:  time.Now().UnixNano(),
		Sender:  username,
	}}); err != nil {
		slog.Debug("sessionhistory.Append (chatter)",
			slog.Int64("chat", in.ChatID),
			slog.String("error", err.Error()))
	}
}

func run(ctx context.Context, b *Bot, in go_bot_telegram.Input, attachInputs []go_bot_telegram.Input) error {
	isCallback := in.CallbackData != "" || len(in.CallbackPicks) > 0
	content := strings.TrimSpace(in.Text)
	if content == "" {
		content = strings.TrimSpace(in.Caption)
	}
	hasAttachment := slices.ContainsFunc(attachInputs, inputHasAttachment)
	if !isCallback && content == "" && !hasAttachment {
		return nil
	}
	if content == "/start" || strings.HasPrefix(content, "/start ") || strings.HasPrefix(content, "/start@") {
		return nil
	}

	if isCallback {
		if b.listener != nil && b.listener.OnCallback(ctx, in.ChatID, in.MessageID, in.CallbackData, in.CallbackPicks) {
			return nil
		}
		return nil
	}

	isPrivate := in.Raw == nil || in.Raw.Message == nil || in.Raw.Message.Chat.Type == models.ChatTypePrivate
	_, hasVerifyPending := pending.Get(in.ChatID)
	hasListenerAwait := b.listener != nil && b.listener.IsAwaitingChat(in.ChatID)
	if !isPrivate && !hasVerifyPending && !hasListenerAwait {
		botUsername := strings.TrimSpace(b.client.Status().Username)
		if botUsername == "" {
			return nil
		}
		target := "@" + botUsername
		if !strings.Contains(content, target) {
			recordChatter(in, content)
			return nil
		}
		content = strings.TrimSpace(strings.ReplaceAll(content, target, ""))
		if content == "" && !hasAttachment {
			return nil
		}
	}

	if !utils.IsAuthorized(filesystem.TelegramAuthPath, strconv.FormatInt(in.ChatID, 10)) {
		deleteMsg := func(msgID int, label string) {
			if msgID == 0 {
				return
			}
			if err := b.client.Delete(ctx, in.ChatID, msgID); err != nil {
				slog.Debug("go_bot_telegram.Bot.client.Delete",
					slog.String("label", label),
					slog.String("chat", chatName(in)),
					slog.Int("msg", msgID),
					slog.String("error", err.Error()))
			}
		}

		if p, ok := pending.Get(in.ChatID); ok {
			if strings.TrimSpace(in.Text) == p.Code {
				if err := authorizeChat(in); err != nil {
					return utilsmsg.Trace("authorizeChat", "", err)
				}
				pending.Clear(in.ChatID)
				deleteMsg(p.PromptMsgID, "prompt")
				deleteMsg(in.MessageID, "code")
				return nil
			}
			deleteMsg(p.PromptMsgID, "prompt")
		}
		deleteMsg(in.MessageID, "unverified")
		code, err := utils.GenerateAuthCode()
		if err != nil {
			return utilsmsg.Trace("utils.GenerateAuthCode", "", err)
		}
		slog.Info("Telegram Verification Code",
			slog.String("name", chatName(in)),
			slog.String("code", code))
		execchatbot.NotifyAdmin(ctx, code, "Telegram "+chatName(in))
		prompt, err := b.client.SendInput(ctx, in.ChatID, 0, "Enter the 6-digit verification code printed in the daemon log.")
		if err != nil {
			slog.Warn("go_bot_telegram.Bot.client.SendInput",
				slog.String("chat", chatName(in)),
				slog.String("error", err.Error()))
			return nil
		}
		promptID := 0
		if prompt != nil {
			promptID = prompt.ID
		}
		pending.Set(in.ChatID, code, promptID)
		return nil
	}

	if b.listener != nil && b.listener.OnText(ctx, in.ChatID, in.MessageID, in.Text) {
		return nil
	}

	if hasAttachment {
		if slices.ContainsFunc(attachInputs, inputHasVoice) && !audiotool.STTEnabled() {
			_, _ = b.client.Send(ctx, in.ChatID, in.MessageID, "No speech-to-text model selected · pick one with <code>/model stt</code> first.", go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
			return nil
		}
		var attachments []chatbot.SavedAttachment
		for _, ai := range attachInputs {
			attachments = append(attachments, saveAttachments(ctx, b, ai)...)
		}
		transcripts, paths, err := chatbot.TranscribeSavedAttachments(ctx, attachments)
		if err != nil {
			slog.Debug("transcribeSavedAttachments",
				slog.String("chat", chatName(in)),
				slog.String("error", err.Error()))
			_, _ = b.client.Send(ctx, in.ChatID, in.MessageID, fmt.Sprintf("⚠️ Voice transcription failed\n<code>%s</code>", html.EscapeString(err.Error())), go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
			return nil
		}
		if len(transcripts) > 0 || len(paths) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, transcripts...)
			if len(paths) > 0 {
				lines = append(lines, "[Telegram attachments]")
				for _, p := range paths {
					lines = append(lines, "- "+p)
				}
			}
			content = strings.Join(lines, "\n")
		}
	}

	content, isNew := chatbot.CutNewCommand(content)
	if content == "" {
		return nil
	}

	chatSessionID, err := sessiontelegram.New(in.ChatID)
	if err != nil {
		return utilsmsg.Trace("session.GetTelegramSession", "", err)
	}

	if !isNew && exec.IsRunning(chatSessionID) {
		exec.AppendSteer(chatSessionID, "", content)
		return nil
	}

	replyTo := ""
	if in.MessageID != 0 {
		replyTo = strconv.Itoa(in.MessageID)
	}
	reply := b.newReply(ctx, in.ChatID, chatName(in), chatSessionID, replyTo)
	reply.Status("thinking...")

	return reply.Run(ctx, exec.ExecuteMeta{
		Task:           content,
		Input:          content,
		Sender:         in.Username,
		SessionID:      chatSessionID,
		AllowAll:       false,
		ReplyMessageID: strconv.Itoa(in.MessageID),
	})
}

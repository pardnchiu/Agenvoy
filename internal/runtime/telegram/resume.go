package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessionTelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
)

func (b *Bot) resumeFromPending(sessionID, taskHash string, answers []any) {
	content, err := interactive.LoadResumeMessage(sessionID, taskHash, answers)
	if err != nil {
		slog.Warn("ask_user resume: pending already consumed",
			slog.String("session", sessionID),
			slog.String("task_hash", taskHash))
		return
	}

	chatID, err := lookupChatID(sessionID)
	if err != nil {
		slog.Error("ask_user resume: lookupChatID",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}

	ctx := context.Background()

	markStatus := func(str string) {
		wrapped := fmt.Sprintf("<blockquote expandable>%s</blockquote>", html.EscapeString(str))
		if err := b.client.SendStatus(ctx, chatID, 0, wrapped, go_bot_telegram.WithStatusSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("SendStatus (resume)",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}
	markStatus("resuming…")

	workDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("ask_user resume: UserHomeDir", slog.String("error", err.Error()))
		return
	}

	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	primary, rest, err := exec.ResolveAgent(ctx, agents.DispatcherBot(), agents.Registry(), content, false, sessionID)
	if err != nil {
		b.client.FinishStatus(ctx, chatID)
		errReply := fmt.Sprintf("<blockquote expandable>⚠️ %s</blockquote>", html.EscapeString(err.Error()))
		b.client.Send(ctx, chatID, 0, errReply, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
		return
	}

	execData := exec.ExecData{
		Agent:          primary,
		FallbackAgents: rest,
		WorkDir:        workDir,
		Content:        content,
		ExcludeTools:   tools.TUIOnlyTools,
		ExcludeSkills:  tools.TUIOnlySkills,
		PendingTask:    taskHash,
	}

	sess, err := getSession(ctx, chatID, "user", content, execData, sessionID, "")
	if err != nil {
		slog.Error("ask_user resume: getSession",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}

	events := make(chan agentTypes.Event, 128)
	go func() {
		execCtx := exec.SuppressDcPush(ctx)
		if execErr := exec.Execute(execCtx, execData, sess, events, false); execErr != nil {
			slog.Warn("ask_user resume: exec",
				slog.String("session", sessionID),
				slog.String("error", execErr.Error()))
		}
		close(events)
	}()

	result := utils.FormatChatbotEvent(events, "[Telegram]", sess.ID, markStatus, func(toolName, text string) string {
		return fmt.Sprintf("<code>%s</code>: <code>%s</code>", toolName, text)
	})

	b.client.FinishStatus(ctx, chatID)

	replyText := strings.TrimSpace(tsPrefixRegex.ReplaceAllString(result.ReplyText, ""))
	replyText = sanitizeHTML(replyText)
	if replyText == "" {
		return
	}

	cleanText, photoPaths, docPaths := extractFileMarkers(replyText)
	replyText = cleanText

	model := result.Done.Model
	if model == "" && primary != nil {
		model = primary.Name()
	}
	footer := utils.FormatEventFooter(result.Done.Duration, model, result.Done.Usage)
	hasMedia := len(photoPaths) > 0 || len(docPaths) > 0
	replyText = chatbot.AppendReplyFooter(chatbot.Telegram, replyText, footer, hasMedia, result.ExecErrors)

	for _, c := range chatbot.Chunk(chatbot.Telegram, replyText) {
		if _, err := b.client.Send(ctx, chatID, 0, c, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML)); err != nil {
			slog.Warn("Send (resume)", slog.String("session", sessionID), slog.String("error", err.Error()))
			break
		}
	}

	if len(photoPaths) > 0 || len(docPaths) > 0 {
		go sendAttachments(context.WithoutCancel(ctx), chatID, "resume", photoPaths, docPaths)
	}
}

func lookupChatID(sessionID string) (int64, error) {
	chatStr, err := sessionTelegram.GetChat(sessionID)
	if err != nil {
		return 0, err
	}
	var chatID int64
	if _, err := fmt.Sscanf(strings.TrimSpace(chatStr), "%d", &chatID); err != nil {
		return 0, fmt.Errorf("parse chatID %q: %w", chatStr, err)
	}
	return chatID, nil
}

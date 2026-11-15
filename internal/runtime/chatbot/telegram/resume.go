package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	sessiontelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func (b *Bot) resumeFromPending(sessionID, taskHash string, answers []any) {
	allowAll := interactive.LoadPendingAllowAll(sessionID, taskHash)
	full, history, err := interactive.LoadResumeMessage(sessionID, taskHash, answers)
	if err != nil {
		if chatID, chErr := lookupChatID(sessionID); chErr == nil {
			b.client.Send(context.Background(), chatID, 0, "Pending task already resolved in another session.", nil)
		}
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

	reply := b.newReply(ctx, chatID, "resume", sessionID, interactive.LoadPendingMessageID(sessionID, taskHash))
	reply.Status("resuming...")

	if _, err := sessiontelegram.New(chatID); err != nil {
		slog.Error("ask_user resume: sessiontelegram.New",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return
	}

	if err := reply.Run(ctx, exec.ExecuteMeta{
		InputNoRecord:  full,
		Input:          full,
		Sender:         "user",
		SessionID:      sessionID,
		AllowAll:       allowAll,
		PendingTask:    taskHash,
		HistoryContent: history,
	}); err != nil {
		slog.Debug("ask_user resume: deliver",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func lookupChatID(sessionID string) (int64, error) {
	chatStr, err := sessiontelegram.GetChat(sessionID)
	if err != nil {
		return 0, err
	}
	var chatID int64
	if _, err := fmt.Sscanf(strings.TrimSpace(chatStr), "%d", &chatID); err != nil {
		return 0, utilsmsg.New(utilsmsg.Invalid, "chatID", "parse %q: %w", chatStr, err)
	}
	return chatID, nil
}

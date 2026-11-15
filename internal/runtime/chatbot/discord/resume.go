package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	sessiondiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func (b *Bot) resumeFromPending(sessionID, taskHash string, answers []any) {
	allowAll := interactive.LoadPendingAllowAll(sessionID, taskHash)
	full, history, err := interactive.LoadResumeMessage(sessionID, taskHash, answers)
	if err != nil {
		channelID, chErr := sessiondiscord.GetChannel(sessionID)
		if chErr == nil && strings.TrimSpace(channelID) != "" {
			b.client.Send(context.Background(), strings.TrimSpace(channelID), "", "Pending task already resolved in another session.")
		}
		return
	}

	channelID, err := sessiondiscord.GetChannel(sessionID)
	if err != nil || strings.TrimSpace(channelID) == "" {
		slog.Error("ask_user resume: GetChannel",
			slog.String("session", sessionID),
			slog.String("error", fmt.Sprint(err)))
		return
	}
	channelID = strings.TrimSpace(channelID)

	ctx := context.Background()

	reply := b.newReply(ctx, channelID, "resume", sessionID, interactive.LoadPendingMessageID(sessionID, taskHash))
	reply.Status("resuming...")

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

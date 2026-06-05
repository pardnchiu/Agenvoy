package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessionDiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func (b *Bot) resumeFromPending(sessionID, taskHash string, answers []any) {
	content, err := interactive.LoadResumeMessage(sessionID, taskHash, answers)
	if err != nil {
		slog.Warn("ask_user resume: pending already consumed",
			slog.String("session", sessionID),
			slog.String("task_hash", taskHash))
		return
	}

	channelID, err := sessionDiscord.GetChannel(sessionID)
	if err != nil || strings.TrimSpace(channelID) == "" {
		slog.Error("ask_user resume: GetChannel",
			slog.String("session", sessionID),
			slog.String("error", fmt.Sprint(err)))
		return
	}
	channelID = strings.TrimSpace(channelID)

	ctx := context.Background()

	markStatus := func(str string) {
		if err := b.client.SendStatus(ctx, channelID, "", str); err != nil {
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
		b.client.FinishStatus(ctx, channelID)
		b.client.Send(ctx, channelID, "", fmt.Sprintf("⚠️ %s", err.Error()))
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

	syntheticIn := go_bot_discord.Input{
		ChannelID: channelID,
		Username:  "user",
	}
	sess, err := getSession(ctx, syntheticIn, content, execData)
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

	result := utils.FormatChatbotEvent(events, "[Discord]", sess.ID, markStatus, func(toolName, text string) string {
		return fmt.Sprintf("`%s`: %s", toolName, text)
	})

	b.client.FinishStatus(ctx, channelID)

	replyText := strings.TrimSpace(result.ReplyText)
	if replyText == "" {
		return
	}

	cleanText, attachmentPaths := utils.ExtractFileMarkers(replyText)
	replyText = cleanText

	model := result.Done.Model
	if model == "" && primary != nil {
		model = primary.Name()
	}
	footer := utils.FormatEventFooter(result.Done.Duration, model, result.Done.Usage)
	hasMedia := len(attachmentPaths) > 0
	replyText = chatbot.AppendReplyFooter(chatbot.Discord, replyText, footer, hasMedia, result.ExecErrors)

	for _, part := range chatbot.Chunk(chatbot.Discord, replyText) {
		if _, err := b.client.Send(ctx, channelID, "", part); err != nil {
			slog.Warn("Send (resume)", slog.String("session", sessionID), slog.String("error", err.Error()))
			break
		}
	}

	if len(attachmentPaths) > 0 {
		go sendAttachments(context.WithoutCancel(ctx), b.client, channelID, "resume", "", attachmentPaths)
	}
}

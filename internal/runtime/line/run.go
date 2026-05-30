package line

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	go_bot_line "github.com/pardnchiu/go-bot/line"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func sourceID(in go_bot_line.Input) string {
	switch {
	case in.GroupID != "":
		return in.GroupID
	case in.RoomID != "":
		return in.RoomID
	default:
		return in.UserID
	}
}

func inputHasAttachment(in go_bot_line.Input) bool {
	return in.MessageType != "" && in.MessageType != "text" && in.MessageID != ""
}

func sourceName(in go_bot_line.Input) string {
	if in.Username != "" {
		return in.Username
	}
	switch {
	case in.GroupID != "":
		return "group:" + in.GroupID
	case in.RoomID != "":
		return "room:" + in.RoomID
	default:
		return "user:" + in.UserID
	}
}

func run(ctx context.Context, b *Bot, in go_bot_line.Input, attachInputs []go_bot_line.Input) error {
	content := strings.TrimSpace(in.Text)
	if content == "" {
		for _, ai := range attachInputs {
			if t := strings.TrimSpace(ai.Text); t != "" {
				content = t
				break
			}
		}
	}
	hasAttachment := slices.ContainsFunc(attachInputs, inputHasAttachment)
	if content == "" && !hasAttachment {
		return nil
	}

	target := sourceID(in)
	if target == "" {
		return fmt.Errorf("no source id")
	}

	if !utils.IsAuthorized(filesystem.LineAuthPath, target) {
		if p, ok := pending.Get(target); ok && content == p.Code {
			if err := authorizeSource(target, in.Username); err != nil {
				return fmt.Errorf("authorizeSource: %w", err)
			}
			pending.Clear(target)
			if _, err := b.client.Send(ctx, target, "verified, you can start the conversation."); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send (verified)",
					slog.String("source", target),
					slog.String("error", err.Error()))
			}
			return nil
		}

		code, err := utils.GenerateAuthCode()
		if err != nil {
			return fmt.Errorf("utils.GenerateAuthCode: %w", err)
		}
		slog.Info("LINE Verification Code",
			slog.String("name", sourceName(in)),
			slog.String("code", code))
		pending.Set(target, code, "")
		if _, err := b.client.Send(ctx, target, "please enter the 6-digit verification code to enable the conversation."); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send (verify prompt)",
				slog.String("source", target),
				slog.String("error", err.Error()))
		}
		return nil
	}

	if hasAttachment {
		dir := filepath.Join(filesystem.AgenvoyDir, "download")
		var labels []string
		for _, ai := range attachInputs {
			if !inputHasAttachment(ai) {
				continue
			}
			path, err := b.client.Save(ctx, ai.MessageID, dir)
			if err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/line Bot.Save",
					slog.String("source", target),
					slog.String("messageType", ai.MessageType),
					slog.String("error", err.Error()))
				continue
			}
			if path == "" {
				continue
			}
			label := "- " + path
			if ai.FileName != "" {
				label += " (" + ai.FileName + ")"
			}
			labels = append(labels, label)
		}
		if len(labels) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, "[LINE attachments]")
			lines = append(lines, labels...)
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
		if _, err := b.client.Send(ctx, target, "⚠️ failed to receive the attachment."); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send (attachment failure)",
				slog.String("source", target),
				slog.String("error", err.Error()))
		}
		return nil
	}

	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	sessionID, err := sessionManager.GetLineSession(in.SourceType, in.UserID, in.GroupID, in.RoomID)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetLineSession: %w", err)
	}

	primary, fallbacks, err := exec.ResolveAgent(ctx, agents.Dispatcher(), agents.Registry(), content, false, sessionID)
	if err != nil {
		if _, sendErr := b.client.Send(ctx, target, fmt.Sprintf("⚠️ %s", err.Error())); sendErr != nil {
			slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send (ResolveAgent error reply)",
				slog.String("source", target),
				slog.String("error", sendErr.Error()))
		}
		return fmt.Errorf("ResolveAgent: %w", err)
	}

	execData := exec.ExecData{
		Agent:          primary,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Content:        content,
		AllowAll:       true,
	}

	sess, err := getSession(in, content, execData)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}
	utils.EventLog("[LINE]", agentTypes.Event{}, sess.ID, content)

	events := make(chan agentTypes.Event, 128)
	go func() {
		execCtx := exec.SuppressDcPush(ctx)
		if execErr := exec.Execute(execCtx, execData, sess, events, execData.AllowAll); execErr != nil {
			slog.Warn("exec",
				slog.String("session", sess.ID),
				slog.String("error", execErr.Error()))
		}
		close(events)
	}()

	result := utils.FormatChatbotEvent(events, "[LINE]", sess.ID, func(string) {}, func(toolName, text string) string {
		return fmt.Sprintf("%s: %s", toolName, text)
	})

	replyText, _ := utils.ExtractFileMarkers(strings.TrimSpace(result.ReplyText))
	replyText = strings.TrimSpace(replyText)
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	model := result.Done.Model
	if model == "" && primary != nil {
		model = primary.Name()
	}
	if footer := utils.FormatEventFooter(result.Done.Duration, model, result.Done.Usage); footer != "" {
		replyText = replyText + "\n\n" + footer
	}
	if len(result.ExecErrors) > 0 {
		replyText = replyText + "\n⚠️ " + strings.Join(result.ExecErrors, ", ")
	}

	for _, part := range chunk(replyText) {
		if _, err := b.client.Send(ctx, target, part); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/line Bot.Send",
				slog.String("session", sess.ID),
				slog.String("source", target),
				slog.String("error", err.Error()))
			return nil
		}
	}
	return nil
}

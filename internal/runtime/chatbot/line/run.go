package line

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot"
	go_bot_line "github.com/pardnchiu/go-bot/core/line"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	execchatbot "github.com/pardnchiu/agenvoy/internal/agents/exec/chatbot"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/tools"
	audioTool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
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

func inputHasVoice(in go_bot_line.Input) bool {
	return in.MessageID != "" && (in.MessageType == "audio" || in.MessageType == "video")
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

func isMentioned(in go_bot_line.Input, botID string) bool {
	if in.Raw == nil || botID == "" {
		return false
	}
	tm, ok := in.Raw.Message.(*linebot.TextMessage)
	if !ok || tm.Mention == nil {
		return false
	}
	for _, m := range tm.Mention.Mentionees {
		if m == nil {
			continue
		}
		if m.Type == linebot.MentionedTargetTypeAll || m.UserID == botID {
			return true
		}
	}
	return false
}

func recordChatter(in go_bot_line.Input, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	sessionID, err := sessionManager.GetLineSession(in.UserID, in.GroupID, in.RoomID)
	if err != nil {
		slog.Warn("sessionManager.GetLineSession (chatter)",
			slog.String("source", sourceName(in)),
			slog.String("error", err.Error()))
		return
	}

	username := in.Username
	if username == "" {
		username = "unknown"
	}
	if err := sessionHistory.Append(sessionID, []sessionHistory.Record{{
		Role:    "user",
		Content: content,
		SendAt:  time.Now().UnixNano(),
		Sender:  username,
	}}); err != nil {
		slog.Warn("sessionHistory.Append (chatter)",
			slog.String("source", sourceName(in)),
			slog.String("error", err.Error()))
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

	_, hasVerifyPending := pending.Get(target)
	if (in.GroupID != "" || in.RoomID != "") && !hasVerifyPending {
		status := b.client.Status()
		if !isMentioned(in, status.UserID) {
			recordChatter(in, content)
			return nil
		}
		if status.DisplayName != "" {
			content = strings.TrimSpace(strings.ReplaceAll(content, "@"+status.DisplayName, ""))
			if content == "" && !hasAttachment {
				return nil
			}
		}
	}

	if !utils.IsAuthorized(filesystem.LineAuthPath, target) {
		if p, ok := pending.Get(target); ok && content == p.Code {
			if err := authorizeSource(target, in.Username); err != nil {
				return fmt.Errorf("authorizeSource: %w", err)
			}
			pending.Clear(target)
			if _, err := b.client.Send(ctx, target, "verified, you can start the conversation."); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (verified)",
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
		execchatbot.NotifyAdmin(ctx, code, "LINE "+sourceName(in))
		pending.Set(target, code, "")
		if _, err := b.client.Send(ctx, target, "please enter the 6-digit verification code to enable the conversation."); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (verify prompt)",
				slog.String("source", target),
				slog.String("error", err.Error()))
		}
		return nil
	}

	if hasAttachment {
		if slices.ContainsFunc(attachInputs, inputHasVoice) && !audioTool.STTEnabled() {
			if _, err := b.client.Send(ctx, target, "⚠️ no speech-to-text model selected · pick one with /model stt first."); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (voice disabled)",
					slog.String("source", target),
					slog.String("error", err.Error()))
			}
			return nil
		}

		pathName := map[string]string{}
		var attachments []chatbot.SavedAttachment
		for _, ai := range attachInputs {
			if !inputHasAttachment(ai) {
				continue
			}
			path, err := b.client.Save(ctx, ai.MessageID, filesystem.DownloadDir)
			if err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Save",
					slog.String("source", target),
					slog.String("messageType", ai.MessageType),
					slog.String("error", err.Error()))
				continue
			}
			if path == "" {
				continue
			}
			if ai.FileName != "" {
				pathName[path] = ai.FileName
			}
			attachments = append(attachments, chatbot.SavedAttachment{Path: path, Transcribe: inputHasVoice(ai)})
		}

		transcripts, paths, err := chatbot.TranscribeSavedAttachments(ctx, attachments)
		if err != nil {
			slog.Warn("chatbot.TranscribeSavedAttachments",
				slog.String("source", target),
				slog.String("error", err.Error()))
			if _, sendErr := b.client.Send(ctx, target, fmt.Sprintf("⚠️ voice transcription failed\n%s", err.Error())); sendErr != nil {
				slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (transcribe failure)",
					slog.String("source", target),
					slog.String("error", sendErr.Error()))
			}
			return nil
		}
		if len(transcripts) > 0 || len(paths) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, transcripts...)
			if len(paths) > 0 {
				lines = append(lines, "[LINE attachments]")
				for _, one := range paths {
					label := "- " + one
					if name := pathName[one]; name != "" {
						label += " (" + name + ")"
					}
					lines = append(lines, label)
				}
			}
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
		if _, err := b.client.Send(ctx, target, "⚠️ failed to receive the attachment."); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (attachment failure)",
				slog.String("source", target),
				slog.String("error", err.Error()))
		}
		return nil
	}

	content, isNew := chatbot.CutNewCommand(content)
	if content == "" {
		return nil
	}

	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	sessionID, err := sessionManager.GetLineSession(in.UserID, in.GroupID, in.RoomID)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetLineSession: %w", err)
	}

	if !isNew && exec.IsRunning(sessionID) {
		exec.AppendSteer(sessionID, "", content)
		return nil
	}

	execData := exec.Prepare(exec.ExecuteMeta{
		WorkDir:       workDir,
		Task:          content,
		Input:         content,
		SessionID:     sessionID,
		ExcludeTools:  tools.TUIOnlyTools,
		ExcludeSkills: tools.TUIOnlySkills,
		AllowAll:      true,
		Sender:        sourceName(in),
	})
	utils.EventLog("[LINE]", agentTypes.Event{}, sessionID, content)

	execCtx := exec.SuppressDcPush(ctx)
	events, wait := exec.Stream(execCtx, sessionID, 128, func(stream chan<- agentTypes.Event) error {
		return exec.Run(execCtx, execData, stream, false)
	})

	result := utils.FormatChatbotEvent(events, "[LINE]", sessionID, func(string) {}, func(toolName, text string) string {
		return fmt.Sprintf("%s: %s", toolName, text)
	})
	if execErr := wait(); execErr != nil {
		slog.Warn("exec",
			slog.String("session", sessionID),
			slog.String("error", execErr.Error()))
		if strings.TrimSpace(result.ReplyText) == "" {
			if _, sendErr := b.client.Send(ctx, target, fmt.Sprintf("⚠️ %s", execErr.Error())); sendErr != nil {
				slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send (exec error reply)",
					slog.String("source", target),
					slog.String("error", sendErr.Error()))
			}
			return fmt.Errorf("exec: %w", execErr)
		}
	}

	replyText, _ := utils.ExtractFileMarkers(strings.TrimSpace(result.ReplyText))
	replyText = strings.TrimSpace(replyText)
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	if footer := utils.FormatEventFooter(result.Done.Duration, result.Done.OutputElapsed, result.Done.Model, result.Done.Reasoning, result.Done.Usage); footer != "" {
		replyText = replyText + "\n\n" + footer
	}
	if len(result.ExecErrors) > 0 {
		replyText = replyText + "\n⚠️ " + strings.Join(result.ExecErrors, ", ")
	}

	for _, part := range chunk(replyText) {
		if _, err := b.client.Send(ctx, target, part); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/core/line Bot.Send",
				slog.String("session", sessionID),
				slog.String("source", target),
				slog.String("error", err.Error()))
			return nil
		}
	}
	return nil
}

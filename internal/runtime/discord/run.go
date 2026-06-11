package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	geminiSummary "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/summary"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	sessionDiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func channelName(in go_bot_discord.Input) string {
	if in.ChannelName != "" {
		return in.ChannelName
	}
	return in.Username
}

func run(ctx context.Context, b *Bot, in go_bot_discord.Input) error {
	if b.listener != nil && b.listener.IsAwaitingPrompt(in.ChannelID, in.MessageID) {
		if b.listener.OnCallback(ctx, in.ChannelID, in.MessageID, in.Text, in.CallbackPicks) {
			return nil
		}
	}

	content := strings.TrimSpace(in.Text)
	hasAttachment := len(in.Attachments) > 0
	if content == "" && !hasAttachment {
		return nil
	}

	_, hasPending := pending.Get(in.ChannelID)
	if in.GuildID != "" && !hasPending {
		botID := b.client.Status().UserID
		mentioned := false
		if in.Raw != nil && in.Raw.Message != nil {
			for _, u := range in.Raw.Message.Mentions {
				if u != nil && u.ID == botID {
					mentioned = true
					break
				}
			}
		}
		if !mentioned {
			return nil
		}
		content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", botID), "")
		content = strings.ReplaceAll(content, fmt.Sprintf("<@!%s>", botID), "")
		content = strings.TrimSpace(content)
		if content == "" && !hasAttachment {
			return nil
		}
	}

	if !utils.IsAuthorized(filesystem.DiscordAuthPath, in.ChannelID) {
		deleteMsg := func(msgID, label string) {
			if msgID == "" {
				return
			}
			if err := b.client.Delete(ctx, in.ChannelID, msgID); err != nil {
				slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Delete",
					slog.String("label", label),
					slog.String("channel", channelName(in)),
					slog.String("msg", msgID),
					slog.String("hint", "grant Manage Messages to bot role if 50013"),
					slog.String("error", err.Error()))
			}
		}

		if p, ok := pending.Get(in.ChannelID); ok {
			if content == p.Code {
				if err := authorizeChannel(in); err != nil {
					return fmt.Errorf("authorizeChannel: %w", err)
				}
				pending.Clear(in.ChannelID)
				deleteMsg(p.PromptMsgID, "prompt")
				if in.MessageID != p.PromptMsgID {
					deleteMsg(in.MessageID, "code")
				}
				return nil
			}
			deleteMsg(p.PromptMsgID, "prompt")
			if in.MessageID != p.PromptMsgID {
				deleteMsg(in.MessageID, "unverified")
			}
		} else {
			deleteMsg(in.MessageID, "unverified")
		}

		code, err := utils.GenerateAuthCode()
		if err != nil {
			return fmt.Errorf("utils.GenerateAuthCode: %w", err)
		}
		slog.Info("Discord Verification Code",
			slog.String("name", channelName(in)),
			slog.String("code", code))
		exec.NotifyAdminCode(ctx, code, "Discord "+channelName(in))
		prompt, err := b.client.SendInput(ctx, in.ChannelID, "", "Enter the 6-digit verification code printed in the daemon log.")
		if err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendInput",
				slog.String("channel", channelName(in)),
				slog.String("error", err.Error()))
			return nil
		}
		promptID := ""
		if prompt != nil {
			promptID = prompt.ID
		}
		pending.Set(in.ChannelID, code, promptID)
		return nil
	}

	autoTranscribed := false
	if hasAttachment {
		if hasVoiceAttachment(in) && !config.VoiceEnabled() {
			_, _ = b.client.Send(ctx, in.ChannelID, in.MessageID, "Please enable it with `/enable-voice enable` first.")
			return nil
		}
		attachments := saveAttachments(ctx, b, in)
		transcripts, paths, err := chatbot.TranscribeSavedAttachments(ctx, attachments)
		if err != nil {
			slog.Warn("transcribeSavedAttachments",
				slog.String("channel", channelName(in)),
				slog.String("error", err.Error()))
			_, _ = b.client.Send(ctx, in.ChannelID, in.MessageID, fmt.Sprintf("⚠️ Voice transcription failed\n`%s`", err.Error()))
			return nil
		}
		if len(transcripts) > 0 || len(paths) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, transcripts...)
			autoTranscribed = len(transcripts) > 0
			if len(paths) > 0 {
				lines = append(lines, "[Discord attachments]")
				for _, p := range paths {
					lines = append(lines, "- "+p)
				}
			}
			content = strings.Join(lines, "\n")
		}
	}

	if content == "" {
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

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(content)
	if externalAgent != "" {
		content = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *skill.Skill
	if externalAgent == "" && scanner != nil {
		if m, effective := runtime.MatchSkill(scanner, content, tools.TUIOnlySkills...); m != nil {
			matchedSkill = m
			content = strings.TrimSpace(effective)
		}
	}

	discordSessionID, err := sessionDiscord.New(in.GuildID, in.ChannelID, in.UserID)
	if err != nil {
		return fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetDiscordSession: %w", err)
	}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	if externalAgent == "" {
		primary, rest, err := exec.ResolveAgent(ctx, agents.DispatcherBot(), agents.Registry(), content, matchedSkill != nil, discordSessionID)
		if err != nil {
			if _, sendErr := b.client.Send(ctx, in.ChannelID, in.MessageID, fmt.Sprintf("⚠️ %s", err.Error())); sendErr != nil {
				slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Send (ResolveAgent error reply)",
					slog.String("channel", channelName(in)),
					slog.String("error", sendErr.Error()))
			}
			return fmt.Errorf("ResolveAgent: %w", err)
		}
		agent = primary
		fallbacks = rest
	}

	execData := exec.ExecData{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Skill:          matchedSkill,
		Content:        content,
		ExcludeTools:   chatbot.RuntimeExcludeTools(autoTranscribed),
		ExcludeSkills:  tools.TUIOnlySkills,
		AllowAll:       false,
	}

	sess, err := getSession(ctx, in, content, execData)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}
	utils.EventLog("[Discord]", agentTypes.Event{}, sess.ID, content)

	markStatus := func(str string) {
		if err := b.client.SendStatus(ctx, in.ChannelID, in.MessageID, str); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendStatus",
				slog.String("session", sess.ID),
				slog.String("text", str),
				slog.String("channel", channelName(in)),
				slog.String("error", err.Error()))
		}
	}
	markStatus("thinking…")

	events := make(chan agentTypes.Event, 128)
	go func() {
		var execErr error
		execCtx := exec.SuppressDcPush(ctx)
		if externalAgent != "" {
			execErr = exec.CallExternal(execCtx, sess.ID, externalAgent, content, externalReadOnly, events)
		} else {
			execErr = exec.Execute(execCtx, execData, sess, events, execData.AllowAll)
		}
		if execErr != nil {
			slog.Warn("exec",
				slog.String("session", sess.ID),
				slog.String("error", execErr.Error()))
		}
		close(events)
	}()

	result := utils.FormatChatbotEvent(events, "[Discord]", sess.ID, markStatus, func(toolName, text string) string {
		return fmt.Sprintf("`%s`: %s", toolName, text)
	})
	replyText := result.ReplyText
	execErrors := result.ExecErrors
	doneEvent := result.Done

	if err := b.client.FinishStatus(ctx, in.ChannelID); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.FinishStatus",
			slog.String("session", sess.ID),
			slog.String("channel", channelName(in)),
			slog.String("error", err.Error()))
	}

	replyText = strings.TrimSpace(replyText)
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	cleanText, attachmentPaths := utils.ExtractFileMarkers(replyText)
	replyText = cleanText

	voiceResult := chatbot.ExtractVoiceMarkers(replyText, autoTranscribed)
	replyText = voiceResult.CleanText
	voiceTexts := voiceResult.Texts
	autoVoiceReply := voiceResult.AutoReply

	model := doneEvent.Model
	if model == "" && agent != nil {
		model = agent.Name()
	}
	footer := utils.FormatEventFooter(doneEvent.Duration, model, doneEvent.Usage)
	hasMedia := len(attachmentPaths) > 0 || len(voiceTexts) > 0
	replyText = chatbot.AppendReplyFooter(chatbot.Discord, replyText, footer, hasMedia, execErrors)

	chunks := chatbot.Chunk(chatbot.Discord, replyText)
	replyTo := in.MessageID
	var replyMsg *discordgo.Message
	for _, part := range chunks {
		msg, sendErr := b.client.Send(ctx, in.ChannelID, replyTo, part)
		if sendErr != nil {
			slog.Error("github.com/pardnchiu/go-bot/discord Bot.client.Send",
				slog.String("session", sess.ID),
				slog.String("channel", channelName(in)),
				slog.String("error", sendErr.Error()))
			b.client.Send(ctx, in.ChannelID, in.MessageID, fmt.Sprintf("⚠️ send failed: %s", sendErr.Error()))
			break
		}
		replyMsg = msg
		replyTo = ""
	}

	if len(voiceTexts) == 0 && len(attachmentPaths) == 0 {
		return nil
	}

	replyToID := ""
	if replyMsg != nil {
		replyToID = replyMsg.ID
	}

	if len(attachmentPaths) > 0 {
		bgCtx := context.WithoutCancel(ctx)
		channel := channelName(in)
		client := b.client
		paths := attachmentPaths
		go sendAttachments(bgCtx, client, in.ChannelID, channel, replyToID, paths)
	}

	if len(voiceTexts) > 0 {
		bgCtx := context.WithoutCancel(ctx)
		channel := channelName(in)
		channelID := in.ChannelID
		reply := replyToID
		client := b.client
		texts := voiceTexts
		summarizeTexts := autoVoiceReply
		sessID := sess.ID
		go func() {
			sendFailure := func(errMsg string) {
				text := fmt.Sprintf("-# ⎿ ⚠️ SendVoice failed (background)\n-# ⎿ `%s`", errMsg)
				if _, err := client.Send(bgCtx, channelID, reply, text); err != nil {
					slog.Error("github.com/pardnchiu/go-bot/discord Bot.client.Send (notify)",
						slog.String("session", sessID),
						slog.String("channel", channel),
						slog.String("error", err.Error()))
				}
			}
			apiKey := strings.TrimSpace(keychain.Get("GEMINI_API_KEY"))
			if apiKey == "" {
				slog.Error("keychain.Get GEMINI_API_KEY missing",
					slog.String("session", sessID),
					slog.String("channel", channel))
				sendFailure("GEMINI_API_KEY missing")
				return
			}
			for _, text := range texts {
				if summarizeTexts {
					summary, err := geminiSummary.VoiceReply(bgCtx, text)
					if err != nil {
						slog.Warn("gemini summary VoiceReply",
							slog.String("session", sessID),
							slog.String("channel", channel),
							slog.String("error", err.Error()))
						summary = utils.VoiceReplyText(text)
					}
					if strings.TrimSpace(summary) == "" {
						summary = utils.VoiceReplyText(text)
					}
					text = summary
				}
				if strings.TrimSpace(text) == "" {
					continue
				}
				if _, err := client.SendVoice(bgCtx, channelID, reply, text, apiKey); err != nil {
					slog.Error("github.com/pardnchiu/go-bot/discord Bot.client.SendVoice",
						slog.String("session", sessID),
						slog.String("channel", channel),
						slog.String("error", err.Error()))
					sendFailure(err.Error())
				}
			}
		}()
	}

	return nil
}

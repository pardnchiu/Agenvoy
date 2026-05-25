package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var voiceMarkerRegex = regexp.MustCompile(`\[SEND_VOICE:([^\]]+)\]`)

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

	if hasAttachment {
		paths := saveAttachments(ctx, b, in)
		if len(paths) > 0 {
			var lines []string
			if content != "" {
				lines = append(lines, content)
			}
			lines = append(lines, "[Discord attachments]")
			for _, p := range paths {
				lines = append(lines, "- "+p)
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

	var matchedSkill *filesystem.Skill
	if externalAgent == "" && scanner != nil {
		if m, effective := runtime.MatchSkill(scanner, content); m != nil {
			matchedSkill = m
			content = strings.TrimSpace(effective)
		}
	}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	if externalAgent == "" {
		primary, rest, err := exec.ResolveAgent(ctx, agents.Dispatcher(), agents.Registry(), content, matchedSkill != nil, "")
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
		AllowAll:       false,
	}

	sess, err := getSession(in, content, execData)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}
	utils.EventLog("[Discord]", agentTypes.Event{}, sess.ID, content)

	markStatus := func(text string) {
		if err := b.client.SendStatus(ctx, in.ChannelID, in.MessageID, text); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendStatus",
				slog.String("session", sess.ID),
				slog.String("text", text),
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

	result := utils.FormatAgentEventMessage(events, "[Discord]", sess.ID, markStatus, func(toolName, text string) string {
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

	var voiceTexts []string
	for _, match := range voiceMarkerRegex.FindAllStringSubmatch(replyText, -1) {
		if t := strings.TrimSpace(match[1]); t != "" {
			voiceTexts = append(voiceTexts, t)
		}
	}
	replyText = strings.TrimSpace(voiceMarkerRegex.ReplaceAllString(replyText, ""))

	model := doneEvent.Model
	if model == "" && agent != nil {
		model = agent.Name()
	}
	footer := utils.FormatFooter(doneEvent.Duration, model, doneEvent.Usage)
	if len(attachmentPaths) > 0 || len(voiceTexts) > 0 {
		footer = "🔗 " + footer
	}
	replyText = fmt.Sprintf("%s\n-# ⎿ %s", replyText, footer)
	if len(execErrors) > 0 {
		replyText = fmt.Sprintf("%s\n-# ⎿ ⚠️ %s", replyText, strings.Join(execErrors, ", "))
	}

	chunks := chunk(replyText)
	replyTo := in.MessageID
	var replyMsg *discordgo.Message
	for _, part := range chunks {
		msg, sendErr := b.client.Send(ctx, in.ChannelID, replyTo, part)
		if sendErr != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Send",
				slog.String("session", sess.ID),
				slog.String("channel", channelName(in)),
				slog.String("error", sendErr.Error()))
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

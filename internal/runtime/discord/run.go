package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/skill"
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
	if content == "" {
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
		if content == "" {
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

	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	scanner := host.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(content)
	if externalAgent != "" {
		content = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *skill.Skill
	if externalAgent == "" && scanner != nil {
		if m, effective := scanner.MatchSkillCall(content); m != nil {
			matchedSkill = m
			content = strings.TrimSpace(effective)
			slog.Info("skill", slog.String("skill", m.Name))
		}
	}

	var agent agentTypes.Agent
	if externalAgent == "" {
		agent = exec.SelectAgent(ctx, host.Planner(), host.Registry(), content, matchedSkill != nil, "")
	}

	execData := exec.ExecData{
		Agent:    agent,
		WorkDir:  workDir,
		Skill:    matchedSkill,
		Content:  content,
		AllowAll: false,
	}

	sess, err := getSession(in, content, execData)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}
	utils.EventLog("[Discord]", agentTypes.Event{}, sess.ID, content)

	markStatus := func(text string) {
		if err := b.client.SendStatus(ctx, in.ChannelID, in.MessageID, text); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendStatus",
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
			slog.String("channel", channelName(in)),
			slog.String("error", err.Error()))
	}

	replyText = strings.TrimSpace(replyText)
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	cleanText, attachmentPaths := extractFileMarkers(replyText)
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
	if _, after, ok := strings.Cut(model, "@"); ok {
		model = after
	}
	footer := model
	if doneEvent.Usage != nil {
		footer = fmt.Sprintf("%s | in:%s out:%s", footer, utils.FormatUsage(doneEvent.Usage.Input), utils.FormatUsage(doneEvent.Usage.Output))
	}
	replyText = fmt.Sprintf("%s\n-# ⎿ %s", replyText, footer)
	if len(execErrors) > 0 {
		replyText = fmt.Sprintf("%s\n-# ⎿ ⚠️ %s", replyText, strings.Join(execErrors, ", "))
	}

	replyMsg, sendErr := b.client.Send(ctx, in.ChannelID, in.MessageID, replyText)
	if sendErr != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Send",
			slog.String("channel", channelName(in)),
			slog.String("error", sendErr.Error()))
	}

	if len(voiceTexts) == 0 && len(attachmentPaths) == 0 {
		return nil
	}

	replyToID := ""
	if replyMsg != nil {
		replyToID = replyMsg.ID
	}
	sendFailure := func(label, detail, errMsg string) {
		text := fmt.Sprintf("-# ⎿ ⚠️ %s failed", label)
		if detail != "" {
			text = fmt.Sprintf("%s: `%s`", text, detail)
		}
		text = fmt.Sprintf("%s\n-# ⎿ `%s`", text, errMsg)
		if _, err := b.client.Send(ctx, in.ChannelID, replyToID, text); err != nil {
			slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Send (notify)",
				slog.String("label", label),
				slog.String("error", err.Error()))
		}
	}

	if err := b.client.SendStatus(ctx, in.ChannelID, replyToID, "sending…",
		go_bot_discord.WithStatusEmoji("⚡")); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendStatus",
			slog.String("channel", channelName(in)),
			slog.String("error", err.Error()))
	}

	if len(attachmentPaths) > 0 {
		sendAttachments(ctx, b.client, in.ChannelID, channelName(in), replyToID, attachmentPaths)
	}

	if len(voiceTexts) > 0 {
		apiKey := strings.TrimSpace(keychain.Get("GEMINI_API_KEY"))
		if apiKey == "" {
			slog.Warn("keychain.Get GEMINI_API_KEY missing",
				slog.String("channel", channelName(in)))
			sendFailure("SendVoice", "", "GEMINI_API_KEY missing")
		} else {
			for _, text := range voiceTexts {
				if _, err := b.client.SendVoice(ctx, in.ChannelID, replyToID, text, apiKey); err != nil {
					slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.SendVoice",
						slog.String("error", err.Error()))
					sendFailure("SendVoice", "", err.Error())
				}
			}
		}
	}

	if err := b.client.FinishStatus(ctx, in.ChannelID); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.FinishStatus",
			slog.String("channel", channelName(in)),
			slog.String("error", err.Error()))
	}

	return nil
}

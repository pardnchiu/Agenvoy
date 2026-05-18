package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/skill"
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

	if _, err := b.client.Send(ctx, in.ChannelID, in.MessageID, replyText); err != nil {
		slog.Warn("github.com/pardnchiu/go-bot/discord Bot.client.Send",
			slog.String("channel", channelName(in)),
			slog.String("error", err.Error()))
	}

	return nil
}

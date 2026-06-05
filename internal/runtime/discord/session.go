package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	go_bot_discord "github.com/pardnchiu/go-bot/discord"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionDiscord "github.com/pardnchiu/agenvoy/internal/session/discord"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
)

func getSession(ctx context.Context, in go_bot_discord.Input, content string, data exec.ExecData) (*agentTypes.AgentSession, error) {
	sessionID, err := sessionDiscord.New(in.GuildID, in.ChannelID, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetDiscordSession: %w", err)
	}

	sess := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	oldHistory, maxHistory := sessionHistory.Get(sessionID)
	sess.Histories = oldHistory
	sess.BaseLen = len(oldHistory)

	sess.SystemPrompts = exec.BuildSystemPrompts(data.WorkDir, data.ExtraSystemPrompt, agents.Scanner(), sessionID, data.AllowAll, false, data.ExcludeSkills)
	if summary := summary.GetPrompt(sessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		sess.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	sess.OldHistories = maxHistory
	sess.ToolHistories = []agentTypes.Message{}

	header := fmt.Sprintf("當前時間: %s\n工作目錄: %s\n傳送者: %s\n當前 channel: %s",
		time.Now().Format("2006-01-02 15:04:05"),
		data.WorkDir,
		in.Username,
		channelName(in),
	)
	userText := fmt.Sprintf("---\n%s\n---\n%s", header, strings.TrimSpace(content))

	sess.Histories = append(sess.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	sess.UserInput = agentTypes.Message{
		Role:    "user",
		Content: userText,
	}
	exec.SaveUserInputHistory(ctx, sessionID, userText)

	return sess, nil
}

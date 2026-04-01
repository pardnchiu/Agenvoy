package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	discordCommand "github.com/pardnchiu/agenvoy/internal/discord/command"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
	"github.com/pardnchiu/agenvoy/internal/scheduler"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func New(plannerAgent agentTypes.Agent, agentRegistry agentTypes.AgentRegistry, skillScanner *skill.SkillScanner) (*discordTypes.DiscordBot, error) {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, nil
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	bot := &discordTypes.DiscordBot{
		Session:       session,
		PlannerAgent:  plannerAgent,
		AgentRegistry: agentRegistry,
		SkillScanner:  skillScanner,
	}

	if cronMgr := scheduler.Get(); cronMgr != nil {
		cronMgr.OnCompleted = func(channelID, output string) {
			if output == "" {
				output = "任務完成"
			}
			var content string
			if strings.HasPrefix(output, "error:") {
				content = output
			} else {
				content = wrapScriptOutput(plannerAgent, output)
			}
			if err := Send(bot, channelID, discordTypes.ReplyMessage{Content: content}); err != nil {
				slog.Warn("Send",
					slog.String("error", err.Error()))
			}
		}
	}

	session.AddHandler(interactionCreate)
	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		messageCreate(bot, s, m)
	})
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages | discordgo.IntentMessageContent

	if err := session.Open(); err != nil {
		return nil, fmt.Errorf("open websocket connection: %w", err)
	}

	discordCommand.Create(bot, session)

	clientID := session.State.User.ID
	oauthURL := fmt.Sprintf(
		"https://discord.com/oauth2/authorize?client_id=%s&scope=identify+email+bot+applications.commands&permissions=83968",
		clientID,
	)
	slog.Info("bot is running",
		slog.String("user", session.State.User.Username))
	fmt.Printf("URL: %s\n", oauthURL)

	return bot, nil
}

func wrapScriptOutput(agent agentTypes.Agent, output string) string {
	if agent == nil {
		return output
	}
	if err := json.Unmarshal([]byte(output), &struct{}{}); err != nil {
		return output
	}
	messages := []agentTypes.Message{
		{
			Role:    "system",
			Content: "你是一個訊息整理助理。收到腳本執行結果後，將其轉化為自然、簡潔、適合在 Discord 頻道傳送的訊息。若結果為空或無意義，回覆「任務已完成」。直接輸出訊息內容，不要加任何前綴或解釋。",
		},
		{
			Role:    "user",
			Content: output,
		},
	}
	resp, err := agent.Send(context.Background(), messages, nil)
	if err != nil || len(resp.Choices) == 0 {
		slog.Warn("wrapScriptOutput: agent.Send failed, using raw output",
			slog.String("error", func() string {
				if err != nil {
					return err.Error()
				}
				return "empty choices"
			}()))
		return output
	}
	if text, ok := resp.Choices[0].Message.Content.(string); ok && text != "" {
		return text
	}
	return output
}

func Close(b *discordTypes.DiscordBot) error {
	slog.Info("shutting down")
	if b.Session == nil {
		return nil
	}
	return b.Session.Close()
}

package discord

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/bwmarrin/discordgo"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/cron"
	discordCommand "github.com/pardnchiu/agenvoy/internal/discord/command"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
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

	if err := cron.New(); err != nil {
		slog.Warn("cron.New",
			slog.String("error", err.Error()))
	} else if err := cron.Get().Load(); err != nil {
		slog.Warn("cron.Get",
			slog.String("error", err.Error()))
	}

	bot := &discordTypes.DiscordBot{
		Session:       session,
		PlannerAgent:  plannerAgent,
		AgentRegistry: agentRegistry,
		SkillScanner:  skillScanner,
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

func Close(b *discordTypes.DiscordBot) error {
	slog.Info("shutting down")
	cron.Stop()
	if b.Session == nil {
		return nil
	}
	return b.Session.Close()
}

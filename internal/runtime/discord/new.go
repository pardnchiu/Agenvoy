package discord

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	discordCommand "github.com/pardnchiu/agenvoy/internal/runtime/discord/command"
	discordTypes "github.com/pardnchiu/agenvoy/internal/runtime/discord/types"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

const Key = "DISCORD_TOKEN"

func New() (*discordTypes.DiscordBot, error) {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.DiscordEnabled {
		return nil, nil
	}
	token := keychain.Get(Key)
	if token == "" {
		return nil, nil
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	bot := &discordTypes.DiscordBot{
		Session: session,
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
	username := session.State.User.Username
	saveDiscordUsername(username)
	fmt.Printf("URL: %s\n", oauthURL)

	return bot, nil
}

func saveDiscordUsername(name string) {
	cfg, err := session.Load()
	if err != nil || cfg == nil || cfg.DiscordUsername == name {
		return
	}
	cfg.DiscordUsername = name
	if err := session.Save(cfg); err != nil {
		slog.Warn("github.com/pardnchiu/agenvoy/internal/session Save",
			slog.String("error", err.Error()))
	}
}

func Close(b *discordTypes.DiscordBot) error {
	if b.Session == nil {
		return nil
	}
	return b.Session.Close()
}

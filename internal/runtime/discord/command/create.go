package discordCommand

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
	discordTypes "github.com/pardnchiu/agenvoy/internal/runtime/discord/types"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func Create(dcBot *discordTypes.DiscordBot, dcSession *discordgo.Session) {
	var command []*discordgo.ApplicationCommand
	for _, cmd := range commands {
		switch cmd {
		case CmdHelp:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Show how to use",
			})
		case CmdRole:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Assign role session to handle",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "name",
						Description: "Role name",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "message",
						Description: "Message",
						Required:    true,
					},
				},
			})
		case CmdAddGemini:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Add Gemini API key",
			})
		case CmdAddOpenAI:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Add OpenAI API key",
			})
		case CmdAddClaude:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Add Claude API key",
			})
		case CmdAddNim:
			command = append(command, &discordgo.ApplicationCommand{
				Name:        cmd.Text(),
				Description: "Add NIM API key",
			})
		}
	}

	guildID := ""
	if cfg, err := session.Load(); err == nil {
		guildID = cfg.DiscordGuildID
	}
	registered, err := dcSession.ApplicationCommandBulkOverwrite(dcSession.State.User.ID, guildID, command)
	if err != nil {
		slog.Warn("failed to register commands",
			slog.String("error", err.Error()))
		return
	}
	dcBot.Commands = append(dcBot.Commands, registered...)
}

package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordCommand "github.com/pardnchiu/agenvoy/internal/interactive/discord/command"
	discordTypes "github.com/pardnchiu/agenvoy/internal/interactive/discord/types"
)

func interactionCreate(dcSession *discordgo.Session, dcInteractionCreate *discordgo.InteractionCreate) {
	switch dcInteractionCreate.Type {
	case discordgo.InteractionApplicationCommand:
		handleSlashCommand(dcSession, dcInteractionCreate)
	case discordgo.InteractionModalSubmit:
		handleModalSubmit(dcSession, dcInteractionCreate)
	}
}

func handleSlashCommand(dcSession *discordgo.Session, dcInteractionCreate *discordgo.InteractionCreate) {
	data := dcInteractionCreate.ApplicationCommandData()

	// * add-key commands: respond with modal, no deferral
	if discordCommand.IsAddKeyCmd(data.Name) {
		dcSession.InteractionRespond(dcInteractionCreate.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "modal_" + data.Name,
				Title:    "Add API Key",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "api_key",
								Label:       "API Key",
								Style:       discordgo.TextInputShort,
								Placeholder: "Paste your API key here",
								Required:    true,
								MinLength:   1,
							},
						},
					},
				},
			},
		})
		return
	}

	var userID, username string
	if dcInteractionCreate.Member != nil {
		userID = dcInteractionCreate.Member.User.ID
		username = dcInteractionCreate.Member.User.Username
	} else if dcInteractionCreate.User != nil {
		userID = dcInteractionCreate.User.ID
		username = dcInteractionCreate.User.Username
	}

	var params []string
	for _, opt := range data.Options {
		params = append(params, opt.StringValue())
	}

	message := &discordTypes.ReceiveMessage{
		GuildID:    dcInteractionCreate.GuildID,
		ChannelID:  dcInteractionCreate.ChannelID,
		AuthorID:   userID,
		AuthorName: username,
		Content:    fmt.Sprintf("/%s %s", data.Name, strings.Join(params, " ")),
		Cmd:        fmt.Sprintf("/%s", data.Name),
		Params:     params,
		IsChannel:  dcInteractionCreate.GuildID != "",
		IsMention:  false,
		RecievedAt: time.Now().Unix(),
	}
	ctx := context.Background()
	dcSession.InteractionRespond(dcInteractionCreate.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	slog.Info("command received",
		slog.String("user", message.AuthorName),
		slog.String("content", message.Content),
		slog.Bool("is_channel", message.IsChannel))

	replies := discordCommand.Handler(message)
	for _, reply := range replies {
		dcReply := &discordTypes.DiscordReply{
			Session:     dcSession,
			Interaction: dcInteractionCreate,
		}
		Reply(ctx, dcReply, reply)
	}
}

func handleModalSubmit(dcSession *discordgo.Session, dcInteractionCreate *discordgo.InteractionCreate) {
	data := dcInteractionCreate.ModalSubmitData()

	var apiKey string
	for _, comp := range data.Components {
		if row, ok := comp.(*discordgo.ActionsRow); ok {
			for _, inner := range row.Components {
				if ti, ok := inner.(*discordgo.TextInput); ok && ti.CustomID == "api_key" {
					apiKey = strings.TrimSpace(ti.Value)
				}
			}
		}
	}

	result := discordCommand.ModalHandler(data.CustomID, apiKey)

	dcSession.InteractionRespond(dcInteractionCreate.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: result,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

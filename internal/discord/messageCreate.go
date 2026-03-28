package discord

import (
	"context"
	"log/slog"
	"regexp"
	"slices"

	"github.com/bwmarrin/discordgo"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
)

func messageCreate(bot *discordTypes.DiscordBot, dcSession *discordgo.Session, dcMessageCreate *discordgo.MessageCreate) {
	if dcMessageCreate.Author.Bot {
		return
	}

	var imageInputs []string
	var fileInputs []discordTypes.FileInput
	for _, attachment := range dcMessageCreate.Attachments {
		if attachment.Width > 0 {
			imageInputs = append(imageInputs, attachment.URL)
		} else {
			fileInputs = append(fileInputs, discordTypes.FileInput{
				Name: attachment.Filename,
				URL:  attachment.URL,
			})
		}
	}

	message := &discordTypes.ReceiveMessage{
		GuildID:     dcMessageCreate.GuildID,
		ChannelID:   dcMessageCreate.ChannelID,
		AuthorID:    dcMessageCreate.Author.ID,
		AuthorName:  dcMessageCreate.Author.Username,
		Content:     dcMessageCreate.Content,
		ImageInputs: imageInputs,
		FileInputs:  fileInputs,
		Cmd:         "",
		Params:      nil,
		IsChannel:   dcMessageCreate.GuildID != "",
		IsMention:   false,
		RecievedAt:  dcMessageCreate.Timestamp.Unix(),
	}

	// * skipped the sticker input
	regex := regexp.MustCompile(`^http(s)?://klipy`)
	if regex.MatchString(dcMessageCreate.Content) {
		slog.Info("klipy link received, ignoring",
			slog.String("content", dcMessageCreate.Content))
		return
	}

	if dcMessageCreate.GuildID != "" {
		botID := dcSession.State.User.ID
		for _, u := range dcMessageCreate.Mentions {
			if u.ID == botID {
				message.IsMention = true
				break
			}
		}

		if !message.IsMention {
			member, err := dcSession.GuildMember(dcMessageCreate.GuildID, botID)
			if err == nil {
				for _, roleID := range dcMessageCreate.MentionRoles {
					if slices.Contains(member.Roles, roleID) {
						message.IsMention = true
						break
					}
				}
			}
		}
	}

	// * if in channel, must be used mention to trigger
	if message.IsChannel && !message.IsMention {
		return
	}

	// * without timeout, to ensure the message will be processed like command
	ctx := context.Background()
	if message.Cmd == "" && bot.PlannerAgent != nil {
		dcSession.MessageReactionAdd(dcMessageCreate.ChannelID, dcMessageCreate.ID, "🦍")
		go func() {
			if err := run(ctx, bot, dcSession, dcMessageCreate, message); err != nil {
				slog.Warn("run",
					slog.String("error", err.Error()))
			}
			dcSession.MessageReactionRemove(dcMessageCreate.ChannelID, dcMessageCreate.ID, "🦍", "@me")
		}()
	}
}

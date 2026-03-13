package discord

import (
	"context"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
)

const (
	// * if content over 2000, split into multiple messages
	replayMax = 2000
	// * discord attachment limit
	attachMax = 10
)

func Send(bot *discordTypes.DiscordBot, channelID string, reply discordTypes.ReplyMessage) error {
	var embeds []*discordgo.MessageEmbed
	if reply.ImageURL != "" {
		embeds = []*discordgo.MessageEmbed{
			{Image: &discordgo.MessageEmbedImage{URL: reply.ImageURL}},
		}
	}

	var files []*discordgo.File
	for _, path := range reply.FilePaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()
		files = append(files, &discordgo.File{
			Name:   filepath.Base(path),
			Reader: f,
		})
	}

	chunks := split(reply.Content)
	replyFiles := chunkFiles(files, attachMax)

	for i, chunk := range chunks {
		var chunkEmbeds []*discordgo.MessageEmbed
		var replyFile []*discordgo.File
		if i == len(chunks)-1 {
			chunkEmbeds = embeds
			if len(replyFiles) > 0 {
				replyFile = replyFiles[0]
				replyFiles = replyFiles[1:]
			}
		}
		_, err := bot.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: chunk,
			Embeds:  chunkEmbeds,
			Files:   replyFile,
		})
		if err != nil {
			return err
		}
	}

	for _, replyFile := range replyFiles {
		_, err := bot.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Files: replyFile,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func Reply(ctx context.Context, dcReply *discordTypes.DiscordReply, reply discordTypes.ReplyMessage) error {
	var embeds []*discordgo.MessageEmbed

	if reply.ImageURL != "" {
		embeds = []*discordgo.MessageEmbed{
			{
				Image: &discordgo.MessageEmbedImage{
					URL: reply.ImageURL,
				},
			},
		}
	}

	var files []*discordgo.File
	for _, path := range reply.FilePaths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()
		files = append(files, &discordgo.File{
			Name:   filepath.Base(path),
			Reader: f,
		})
	}

	if dcReply.Interaction != nil {
		replyFiles := chunkFiles(files, attachMax)
		files := []*discordgo.File(nil)
		if len(replyFiles) > 0 {
			files = replyFiles[0]
			replyFiles = replyFiles[1:]
		}
		_, err := dcReply.Session.FollowupMessageCreate(dcReply.Interaction.Interaction, true, &discordgo.WebhookParams{
			Content: reply.Content,
			Embeds:  embeds,
			Files:   files,
		})
		if err != nil {
			return err
		}

		for _, replyFile := range replyFiles {
			_, err := dcReply.Session.FollowupMessageCreate(dcReply.Interaction.Interaction, true, &discordgo.WebhookParams{
				Files: replyFile,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}

	chunks := split(reply.Content)
	replyFiles := chunkFiles(files, attachMax)

	for i, chunk := range chunks {
		var ref *discordgo.MessageReference
		if i == 0 {
			ref = dcReply.Reference
		}
		var chunkEmbeds []*discordgo.MessageEmbed
		var replyFile []*discordgo.File
		if i == len(chunks)-1 {
			chunkEmbeds = embeds
			if len(replyFiles) > 0 {
				replyFile = replyFiles[0]
				replyFiles = replyFiles[1:]
			}
		}
		_, err := dcReply.Session.ChannelMessageSendComplex(dcReply.ChannelID, &discordgo.MessageSend{
			Content:   chunk,
			Reference: ref,
			Embeds:    chunkEmbeds,
			Files:     replyFile,
		})
		if err != nil {
			return err
		}
	}

	// * over 10 files
	for _, replyFile := range replyFiles {
		_, err := dcReply.Session.ChannelMessageSendComplex(dcReply.ChannelID, &discordgo.MessageSend{
			Files: replyFile,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func split(s string) []string {
	if len(s) <= replayMax {
		return []string{s}
	}
	var chunks []string
	for len(s) > replayMax {
		cut := replayMax
		if idx := isLast(s[:cut]); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		chunks = append(chunks, s)
	}
	return chunks
}

func chunkFiles(files []*discordgo.File, size int) [][]*discordgo.File {
	if len(files) == 0 {
		return nil
	}
	var chunkFiles [][]*discordgo.File
	for size < len(files) {
		files, chunkFiles = files[size:], append(chunkFiles, files[:size])
	}
	return append(chunkFiles, files)
}

func isLast(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}

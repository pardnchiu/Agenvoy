package discordTypes

import (
	"github.com/bwmarrin/discordgo"
)

type DiscordBot struct {
	Session  *discordgo.Session
	Commands []*discordgo.ApplicationCommand
}

type FileInput struct {
	Name string
	URL  string
}

type ReceiveMessage struct {
	GuildID     string
	ChannelID   string
	AuthorID    string
	AuthorName  string
	Content     string
	ImageInputs []string
	FileInputs  []FileInput
	Cmd         string
	Params      []string
	IsChannel   bool
	IsMention   bool
	RecievedAt  int64
}

type DiscordReply struct {
	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	ChannelID   string
	Reference   *discordgo.MessageReference
}

type ReplyMessage struct {
	Content   string
	ImageURL  string
	FilePaths []string
}

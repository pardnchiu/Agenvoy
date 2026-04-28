package discordCommand

import (
	"fmt"
	"log/slog"

	discordTypes "github.com/pardnchiu/agenvoy/internal/interactive/discord/types"
)

func Handler(receiveMessage *discordTypes.ReceiveMessage) []discordTypes.ReplyMessage {
	var replies []discordTypes.ReplyMessage

	slog.Info("handler",
		slog.String("cmd", receiveMessage.Cmd),
		slog.Any("params", receiveMessage.Params),
		slog.String("content", receiveMessage.Content),
	)

	if receiveMessage.Cmd != "" {
		switch getCmd(receiveMessage.Cmd) {
		case CmdHelp:
			return []discordTypes.ReplyMessage{
				{Content: "is in building"},
			}
		case CmdRole:
			if len(receiveMessage.Params) < 2 {
				return []discordTypes.ReplyMessage{
					{Content: "Usage: `/role {name} {message}`"},
				}
			}
			role := receiveMessage.Params[0]
			message := receiveMessage.Params[1]
			return []discordTypes.ReplyMessage{
				{Content: fmt.Sprintf("Assign role: %s, message: %s (is in building)", role, message)},
			}
		}
	}

	return replies
}

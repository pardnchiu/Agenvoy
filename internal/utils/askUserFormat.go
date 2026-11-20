package utils

import (
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/locale"
)

type AskMessage struct {
	Title    string
	Detail   string
	Progress string
}

func FormatAskUser(question, detail string, index, total int) AskMessage {
	msg := AskMessage{
		Title:  strings.TrimSpace(question),
		Detail: strings.TrimSpace(detail),
	}
	if total > 1 {
		msg.Progress = fmt.Sprintf(locale.GetAskUser().Progress, index+1, total)
	}
	return msg
}

func FormatChatAsk(msg AskMessage) string {
	parts := []string{msg.Title}
	if msg.Detail != "" {
		parts = append(parts, msg.Detail)
	}
	if msg.Progress != "" {
		parts = append(parts, msg.Progress)
	}
	return strings.Join(parts, "\n")
}

func FormatDiscordAskText(header string, secret bool) string {
	if secret {
		return header + "\n\n" + locale.GetAskUser().DiscordSecret
	}
	return header
}

func FormatTelegramAskText(header string, secret bool) string {
	if secret {
		return header + "\n\n" + locale.GetAskUser().TelegramSecret
	}
	return header
}

package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func PushDiscordResult(ctx context.Context, payload exec.PushPayload) {
	id := strings.TrimSpace(payload.SessionID)
	text := strings.TrimSpace(payload.Text)
	if id == "" || text == "" || !strings.HasPrefix(id, "dc-") {
		return
	}

	channelID, err := sessionManager.GetChannelID(id)
	if err != nil {
		slog.Warn("sessionManager.GetChannelID",
			slog.String("session_id", id),
			slog.String("error", err.Error()))
		return
	}
	if channelID == "" {
		return
	}

	token := strings.TrimSpace(keychain.Get(Key))
	if token == "" {
		slog.Warn("keychain.Get",
			slog.String("session_id", id))
		return
	}

	dgSession, err := discordgo.New("Bot " + token)
	if err != nil {
		slog.Warn("discordgo.New",
			slog.String("error", err.Error()))
		return
	}

	message := text + buildFooter(payload.Model, payload.Usage)
	if prefix := strings.TrimSpace(payload.Prefix); prefix != "" {
		message = "[" + prefix + "]\n" + message
	}
	if _, err := dgSession.ChannelMessageSend(channelID, message); err != nil {
		slog.Warn("dgSession.ChannelMessageSend",
			slog.String("channel_id", channelID),
			slog.String("error", err.Error()))
	}
	_ = ctx
}

func buildFooter(model string, usage *agentTypes.Usage) string {
	model = strings.TrimSpace(model)
	if model == "" && usage == nil {
		return ""
	}
	if _, after, ok := strings.Cut(model, "@"); ok {
		model = after
	}
	footer := model
	if usage != nil {
		if footer != "" {
			footer = fmt.Sprintf("%s | in:%d out:%d", footer, usage.Input, usage.Output)
		} else {
			footer = fmt.Sprintf("in:%d out:%d", usage.Input, usage.Output)
		}
	}
	return "\n-# " + footer
}

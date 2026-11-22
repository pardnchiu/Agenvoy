package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	go_bot_discord "github.com/pardnchiu/go-bot/core/discord"
	go_bot_telegram "github.com/pardnchiu/go-bot/core/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot/discord"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot/telegram"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registSendToChatbot() {
	toolregister.Regist(toolregister.Def{
		Name:        "send_to_chatbot",
		Concurrent:  true,
		Description: sendToChatbotDescription(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"platform": platformParam(),
				"target_id": map[string]any{
					"type":        "string",
					"description": "Chat/channel id (from list_chatbot).",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Formatted message body (HTML for Telegram, markdown for Discord).",
				},
			},
			"required": []string{"platform", "target_id", "message"},
		},
		Handler: func(ctx context.Context, _ *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Platform string `json:"platform"`
				TargetID string `json:"target_id"`
				Message  string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			platform, err := parsePlatform(strings.TrimSpace(params.Platform))
			if err != nil {
				return "", err
			}
			targetID := strings.TrimSpace(params.TargetID)
			message := strings.TrimSpace(params.Message)
			if targetID == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "target_id", "is required")
			}
			if message == "" {
				return "", utilsmsg.New(utilsmsg.Invalid, "message", "is required")
			}

			switch platform {
			case platformTelegram:
				return sendTelegram(ctx, targetID, message)
			case platformDiscord:
				return sendDiscord(ctx, targetID, message)
			}
			return "", utilsmsg.New(utilsmsg.Internal, "platform", "unreachable %q", platform)
		},
	})
}

func sendToChatbotDescription() string {
	var sb strings.Builder
	sb.WriteString("Send a formatted message to an authorized chat/channel, from any session (including TUI / CLI / cron). Never fabricate target_id — call list_chatbot for the platform first.\n")
	if slices.Contains(platformEnum, platformTelegram) {
		sb.WriteString("- Telegram (platform=telegram): if the user did not name a specific chat, list_chatbot(platform=telegram) → ask_user(options=[names]) → map chosen name → target_id → send. Group ids carrying a `-` prefix are especially prone to LLM hallucination and may target chats the bot was kicked from (→ 403 forbidden).\n")
		sb.WriteString("- Telegram messages go out with parse_mode=HTML: use <b>, <i>, <code>, <pre>, <a href>, <blockquote> and nothing else. Markdown (**bold**, backtick code, leading -/*/#, [text](url), fenced blocks) and `| ... |` tables render as literal characters.\n")
	}
	if slices.Contains(platformEnum, platformDiscord) {
		sb.WriteString("- Discord (platform=discord): if the user did not name a specific channel, list_chatbot(platform=discord) → ask_user(options=[names]) → map chosen name → target_id → send.\n")
		sb.WriteString("- Discord messages are markdown: **bold**, *italic*, backtick code, fenced blocks, > quote, - bullet, # heading (H1-H3). HTML tags, LaTeX and tables render as literal characters.\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func sendTelegram(ctx context.Context, chatIDStr, message string) (string, error) {
	if !utils.IsAuthorized(filesystem.TelegramAuthPath, chatIDStr) {
		return "", utilsmsg.New(utilsmsg.Denied, "chat_id", "%q is not authorized; call list_chatbot with platform=telegram", chatIDStr)
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Invalid, "strconv.ParseInt", "", err)
	}

	token := strings.TrimSpace(keychain.Get(telegram.Key))
	if token == "" {
		return "", utilsmsg.New(utilsmsg.Denied, "keychain", "entry %q missing; enable Telegram via TUI /telegram", telegram.Key)
	}

	client, err := go_bot_telegram.New(token,
		go_bot_telegram.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}),
	)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.New", "", err)
	}

	msg, err := client.Send(ctx, chatID, 0, message, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_telegram.Send", "", err)
	}
	recordOutbound(resolveChatbotSession("tg-", "chat_id", chatIDStr), message)

	raw, err := json.Marshal(map[string]any{
		"ok":         true,
		"chat_id":    chatIDStr,
		"message_id": msg.ID,
	})
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}

func sendDiscord(ctx context.Context, channelID, message string) (string, error) {
	if !utils.IsAuthorized(filesystem.DiscordAuthPath, channelID) {
		return "", utilsmsg.New(utilsmsg.Denied, "channel_id", "%q is not authorized; call list_chatbot with platform=discord", channelID)
	}

	token := strings.TrimSpace(keychain.Get(discord.Key))
	if token == "" {
		return "", utilsmsg.New(utilsmsg.Denied, "keychain", "entry %q missing; enable Discord via TUI /discord", discord.Key)
	}

	client, err := go_bot_discord.New(token)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_discord.New", "", err)
	}

	msg, err := client.Send(ctx, channelID, "", message)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "go_bot_discord.Send", "", err)
	}
	recordOutbound(resolveChatbotSession("dc-", "channel_id", channelID), message)

	raw, err := json.Marshal(map[string]any{
		"ok":         true,
		"channel_id": channelID,
		"message_id": msg.ID,
	})
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}

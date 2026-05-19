package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	go_bot_telegram "github.com/pardnchiu/go-bot/telegram"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func registSendToTelegramChat() {
	toolRegister.Regist(toolRegister.Def{
		Name:        "send_to_telegram_chat",
		Description: `[system-default] Send an HTML-formatted message to an authorized Telegram chat by chat_id.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"chat_id": map[string]any{
					"type":        "string",
					"description": "Numeric Telegram chat id (from list_telegram_chat).",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "HTML-formatted message body.",
				},
			},
			"required": []string{"chat_id", "message"},
		},
		Handler: func(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				ChatID  string `json:"chat_id"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", fmt.Errorf("json.Unmarshal: %w", err)
			}
			chatIDStr := strings.TrimSpace(params.ChatID)
			message := strings.TrimSpace(params.Message)
			if chatIDStr == "" {
				return "", fmt.Errorf("chat_id is required")
			}
			if message == "" {
				return "", fmt.Errorf("message is required")
			}

			if !utils.IsAuthorized(filesystem.TelegramAuthPath, chatIDStr) {
				return "", fmt.Errorf("chat_id %q is not in %s; call list_telegram_chat for authorized targets", chatIDStr, filesystem.TelegramAuthPath)
			}

			chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
			if err != nil {
				return "", fmt.Errorf("strconv.ParseInt: %w", err)
			}

			token := strings.TrimSpace(keychain.Get(telegram.Key))
			if token == "" {
				return "", fmt.Errorf("keychain entry %q missing; enable Telegram via TUI /telegram", telegram.Key)
			}

			client, err := go_bot_telegram.New(token,
				go_bot_telegram.WithHTTPClient(&http.Client{Timeout: 5 * time.Minute}),
			)
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-bot/telegram New: %w", err)
			}

			msg, err := client.Send(ctx, chatID, 0, message, go_bot_telegram.WithSendType(go_bot_telegram.TypeHTML))
			if err != nil {
				return "", fmt.Errorf("github.com/pardnchiu/go-bot/telegram Bot.Send: %w", err)
			}

			out, err := json.Marshal(map[string]any{
				"ok":         true,
				"chat_id":    chatIDStr,
				"message_id": msg.ID,
			})
			if err != nil {
				return "", fmt.Errorf("json.Marshal: %w", err)
			}
			return string(out), nil
		},
	})
}

package exec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type AdminSendFunc func(ctx context.Context, targetID, str string) error

var (
	adminSenderMu sync.RWMutex
	adminSenders  = map[string]AdminSendFunc{}
)

func RegisterAdminSender(prefix string, fn AdminSendFunc) {
	adminSenderMu.Lock()
	adminSenders[prefix] = fn
	adminSenderMu.Unlock()
}

func ParseAdminChannel(v string) (prefix, id string, ok bool) {
	v = strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(v, "tg@"):
		prefix, id = "tg", strings.TrimSpace(v[len("tg@"):])
	case strings.HasPrefix(v, "dc@"):
		prefix, id = "dc", strings.TrimSpace(v[len("dc@"):])
	default:
		return "", "", false
	}
	if id == "" {
		return "", "", false
	}
	return prefix, id, true
}

func NotifyAdminCode(ctx context.Context, code, sourceName string) {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return
	}
	value := strings.TrimSpace(cfg.AdminChannel)
	if value == "" {
		return
	}

	prefix, id, ok := ParseAdminChannel(value)
	if !ok {
		slog.Warn("admin_channel malformed; verification code stays log-only",
			slog.String("value", value))
		return
	}

	var authPath string
	switch prefix {
	case "tg":
		authPath = filesystem.TelegramAuthPath
	case "dc":
		authPath = filesystem.DiscordAuthPath
	}
	if !utils.IsAuthorized(authPath, id) {
		slog.Warn("admin_channel not in auth file; verification code stays log-only",
			slog.String("channel", value))
		return
	}

	adminSenderMu.RLock()
	send, ok := adminSenders[prefix]
	adminSenderMu.RUnlock()
	if !ok {
		return
	}

	str := fmt.Sprintf("%s requested access · verification code: %s", sourceName, code)
	if err := send(ctx, id, str); err != nil {
		slog.Warn("admin_channel send failed",
			slog.String("channel", value),
			slog.String("error", err.Error()))
	}
}

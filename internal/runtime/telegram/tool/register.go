package tool

import (
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func Register() {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.TelegramEnabled {
		return
	}
	if strings.TrimSpace(keychain.Get(telegram.Key)) == "" {
		return
	}
	registTelegramFormat()
}

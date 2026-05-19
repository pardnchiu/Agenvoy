package tool

import (
	"strings"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func Register() {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.DiscordEnabled {
		return
	}
	if strings.TrimSpace(keychain.Get(discord.Key)) == "" {
		return
	}
	registDiscordFormat()
}

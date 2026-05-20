package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func NewSession(name string) {
	name = strings.TrimSpace(name)
	if name != "" {
		if sid := session.GetSessionIDByName(name); sid != "" {
			slog.Error("Name already used")
			os.Exit(1)
		}
	}

	newID, err := session.CreateSession("cli-")
	if err != nil {
		slog.Error("session.CreateSession",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if name != "" {
		session.SaveBot(newID, name, true)
	}

	if name != "" {
		fmt.Printf("New session: %s (name=%s)\n", utils.ShortenSessionID(newID), name)
	} else {
		fmt.Printf("New session: %s\n", utils.ShortenSessionID(newID))
	}
}

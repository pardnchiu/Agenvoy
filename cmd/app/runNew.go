package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	go_utils_filesystem "github.com/pardnchiu/go-utils/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func runNew(name string) {
	name = strings.TrimSpace(name)
	if name != "" {
		if sid := getSessionIDByName(name); sid != "" {
			slog.Error("Name already used")
			os.Exit(1)
		}
	}

	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	previous := strings.TrimSpace(cfg.SessionID)

	newID, err := session.CreateSession("cli-")
	if err != nil {
		slog.Error("session.CreateSession",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if name != "" {
		session.SaveBot(newID, name, true)
	}

	cfg.SessionID = newID
	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	if name != "" {
		fmt.Printf("New session: %s (name=%s)\n", newID, name)
	} else {
		fmt.Printf("New session: %s\n", newID)
	}
	if previous != "" && previous != newID {
		fmt.Printf("Previous: %s\n", previous)
	}
}

func getSessionIDByName(name string) string {
	if name == "" {
		return ""
	}

	dirs, err := go_utils_filesystem.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return ""
	}

	for _, sid := range dirs {
		if !strings.HasPrefix(sid, "cli-") {
			continue
		}

		botName, _ := session.GetBot(sid)
		if botName == "" {
			continue
		}
		if botName == name {
			return sid
		}
	}
	return ""
}

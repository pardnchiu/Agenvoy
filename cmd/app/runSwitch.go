package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/session"
)

func runSwitch(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		slog.Error("name is required")
		os.Exit(1)
	}

	match := getSessionIDByName(name)
	if match == "" {
		slog.Error("not found")
		os.Exit(1)
	}

	cfg, err := session.Load()
	if err != nil {
		slog.Error("session.Load",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	previous := strings.TrimSpace(cfg.SessionID)

	cfg.SessionID = match
	if err := session.Save(cfg); err != nil {
		slog.Error("session.Save",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Printf("[*] Switched to: %s\n", match)
	if previous != "" && previous != match {
		fmt.Printf("[*] Previous: %s\n", previous)
	}
}

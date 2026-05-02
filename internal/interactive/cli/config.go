package cli

import (
	"fmt"
	"log/slog"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
)

func Config() {
	sessionID, err := getSessionID()
	if err != nil {
		slog.Error("getSessionID",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	session.SaveBot(sessionID, sessionID, false)

	botPath := filepath.Join(filesystem.SessionsDir, sessionID, "bot.md")
	if !go_pkg_filesystem_reader.Exists(botPath) {
		session.SaveBot(sessionID, sessionID, true)
	}

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		slog.Error("EDITOR is empty after trim")
		os.Exit(1)
	}
	args := append(parts[1:], botPath)
	cmd := osexec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Editing %s\n", botPath)
	if err := cmd.Run(); err != nil {
		slog.Error("cmd.Run",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func getSessionID() (string, error) {
	cfg, err := session.Load()
	if err != nil {
		return "", fmt.Errorf("session.Load: %w", err)
	}

	if sid := strings.TrimSpace(cfg.SessionID); sid != "" {
		if go_pkg_filesystem_reader.Exists(filepath.Join(filesystem.SessionsDir, sid)) {
			return sid, nil
		}
	}

	id, err := session.CreateSession("cli-")
	if err != nil {
		return "", fmt.Errorf("session.CreateSession: %w", err)
	}

	cfg.SessionID = id
	if err := session.Save(cfg); err != nil {
		return "", fmt.Errorf("session.Save: %w", err)
	}
	return id, nil
}

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

func Config(name string) {
	name = strings.TrimSpace(name)

	var sessionID string
	if name != "" {
		sessionID = session.GetSessionIDByName(name)
		if sessionID == "" {
			slog.Error("not found")
			os.Exit(1)
		}
	} else {
		if sid, ok := pickSession("Select session to edit"); ok {
			sessionID = sid
		} else {
			id, err := ResolveSession()
			if err != nil {
				slog.Error("ResolveSession",
					slog.String("error", err.Error()))
				os.Exit(1)
			}
			sessionID = id
		}
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

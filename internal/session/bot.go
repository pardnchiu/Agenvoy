package session

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	botFrontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)
	botNameRegex        = regexp.MustCompile(`(?m)^name:\s*(.+)$`)
)

func SaveBot(sessionID, name string, force bool) {
	if sessionID == "" {
		return
	}

	if name == "" {
		name = sessionID
	}

	dir := filepath.Join(filesystem.SessionsDir, sessionID)
	if err := go_pkg_filesystem.CheckDir(dir, true); err != nil {
		slog.Warn("go_pkg_filesystem.CheckDir",
			slog.String("error", err.Error()))
		return
	}

	path := filepath.Join(dir, "bot.md")
	if !force && go_pkg_filesystem_reader.Exists(path) {
		return
	}

	content := fmt.Sprintf("---\nname: %s\n---\n%s", name, configs.DefaultSessionPrompt)
	if err := go_pkg_filesystem.WriteFile(path, content, 0644); err != nil {
		slog.Warn("go_pkg_filesystem.WriteFile",
			slog.String("error", err.Error()))
	}
}

func GetSessionIDByName(name string) string {
	if name == "" {
		return ""
	}

	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return ""
	}

	for _, sid := range dirs {
		if !strings.HasPrefix(sid, "cli-") && !strings.HasPrefix(sid, "http-") {
			continue
		}

		botName, _ := GetBot(sid)
		if botName == "" {
			continue
		}
		if botName == name {
			return sid
		}
	}
	return ""
}

func GetBot(sessionID string) (name, body string) {
	if sessionID == "" {
		return "", ""
	}
	path := filepath.Join(filesystem.SessionsDir, sessionID, "bot.md")
	data, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return "", ""
	}
	if m := botFrontmatterRegex.FindStringSubmatch(data); len(m) >= 3 {
		header := m[1]
		body = strings.TrimSpace(m[2])
		if nm := botNameRegex.FindStringSubmatch(header); len(nm) > 1 {
			name = strings.TrimSpace(nm[1])
		}
		return name, body
	}
	return "", strings.TrimSpace(data)
}

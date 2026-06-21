package tui

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/session"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type SessionNewSubmit struct {
	name string
}

type SessionNewPromptSubmit struct {
	name string
	body string
}

type SessionNewCustomSubmit struct {
	name string
}

func (t TUI) commandNew(parts []string) (TUI, tea.Cmd, bool) {
	if len(parts) >= 2 {
		name := strings.TrimSpace(strings.Join(parts[1:], " "))
		next, cmd := t.showNewPromptPicker(name)
		return next, cmd, true
	}
	t.popup = &Popup{
		kind:  popupText,
		title: "New session name (empty = unnamed)",
		input: "",
		onConfirm: func(value string) any {
			return SessionNewSubmit{name: strings.TrimSpace(value)}
		},
	}
	return t, nil, true
}

func (t TUI) showNewPromptPicker(name string) (TUI, tea.Cmd) {
	options, values := listPromptTemplates()
	if len(options) == 0 {
		return t.runCreateSession(name, "")
	}

	displayOptions := append(options, "Custom")
	displayValues := append(values, "")

	t.popup = &Popup{
		kind:    popupSingleSelect,
		title:   "Session description",
		options: displayOptions,
		values:  displayValues,
		cursor:  0,
		onConfirm: func(chosen string) any {
			if chosen == "" {
				return SessionNewCustomSubmit{name: name}
			}
			return SessionNewPromptSubmit{name: name, body: readPromptTemplate(chosen)}
		},
	}
	return t, nil
}

func (t TUI) showNewCustomPopup(name string) (TUI, tea.Cmd) {
	t.popup = &Popup{
		kind:      popupText,
		title:     "Session description",
		multiline: true,
		input:     "",
		onConfirm: func(value string) any {
			return SessionNewPromptSubmit{name: name, body: strings.TrimSpace(value)}
		},
	}
	return t, nil
}

func (t TUI) runCreateSession(name, body string) (TUI, tea.Cmd) {
	if name != "" {
		if owner := session.GetSessionID(name); owner != "" {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] name %q already used by session %s", name, owner)) + "\n")
		}
	}

	id, err := session.New("cli-")
	if err != nil {
		return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] create session failed: %v", err)) + "\n")
	}

	if name != "" || body != "" {
		if err := configBot.Save(id, name, body, true); err != nil {
			slog.Warn("sessionBot.Save", slog.String("session", id), slog.String("error", err.Error()))
		}
	}

	previous := t.currentSessionID
	t.currentSessionID = id
	t.currentSessionName, _ = configBot.Get(id)

	t.tokens = 0
	t.lastIn = 0
	t.lastOut = 0
	t.currentModel = ""
	t.activity = ""

	if !t.onceCall {
		t = t.restartTailer()
	}

	if t.onceCall {
		return t, nil
	}

	label := utils.ShortenSessionID(id)
	if name != "" {
		label = fmt.Sprintf("%s (%s)", name, label)
	}
	lines := []string{hintStyle.Render(fmt.Sprintf("⎯ new session: %s", label))}
	if previous != "" && previous != id {
		lines = append(lines, hintStyle.Render(fmt.Sprintf("  previous: %s", utils.ShortenSessionID(previous))))
	}

	return t, tea.Sequence(
		tea.ClearScreen,
		tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		tea.Println(strings.Join(lines, "\n")+"\n"),
	)
}

func listPromptTemplates() (options, values []string) {
	dir := filesystem.PromptsDir
	if !go_pkg_filesystem_reader.IsDir(dir) {
		return nil, nil
	}
	files, err := go_pkg_filesystem_reader.ListFiles(dir)
	if err != nil {
		return nil, nil
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".md") {
			continue
		}
		options = append(options, strings.TrimSuffix(f.Name, ".md"))
		values = append(values, filepath.Join(dir, f.Name))
	}
	return options, values
}

func readPromptTemplate(path string) string {
	raw, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(raw)
}

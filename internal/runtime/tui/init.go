package tui

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type TUIMode int

const (
	cliMode TUIMode = iota
	webMode

	historyLoad = 100
)

func (m TUIMode) String() string {
	switch m {
	case cliMode:
		return "cli"
	case webMode:
		return "web"
	}
	return "unknow"
}

func (m TUIMode) color() lipgloss.Color {
	switch m {
	case cliMode:
		return colSystem
	case webMode:
		return colOk
	}
	return colError
}

type TUI struct {
	ctx      context.Context
	textarea textarea.Model
	spinner  spinner.Model

	running      bool
	cancelExec   context.CancelFunc
	runStartedAt time.Time

	popup        *Popup
	popupQueue   []Pending
	botBodyDraft string
	mcpAdd       *mcpAddDraft

	selector *CmdSelector

	currentModel       string
	currentSessionID   string
	currentSessionName string
	activity           string
	lastIn             int
	lastOut            int

	mode TUIMode

	tailCancel context.CancelFunc

	tokens         int
	width          int
	height         int
	cwd            string
	daemonStatus   string
	httpStatus     string
	discordStatus  string
	telegramStatus string
	runTarget      string
	streaming      bool

	inputHistory    []string
	inputHistoryIdx int

	quitting bool
}

func (t TUI) Init() tea.Cmd {
	seq := []tea.Cmd{
		tea.ClearScreen,
		tea.Batch(
			textarea.Blink,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		),
	}
	seq = append(seq, func() tea.Msg { return initTailer{} })
	if sid := strings.TrimSpace(t.currentSessionID); sid != "" {
		path := filepath.Join(filesystem.SessionsDir, sid, "action.log")
		if go_pkg_filesystem_reader.Exists(path) && fileSize(path) > 0 {
			seq = append(seq, func() tea.Msg { return LoadHistoryCheck{id: sid} })
		}
	}
	return tea.Sequence(seq...)
}

type LoadHistoryCheck struct {
	id string
}

type LoadHistorySelect struct {
	id   string
	load bool
}

func newModel(ctx context.Context) TUI {
	textArea := textarea.New()
	textArea.Placeholder = `Ask anything — research, planning, daily — or type / for commands`
	textArea.CharLimit = 8000
	textArea.SetHeight(1)
	textArea.ShowLineNumbers = false
	textArea.Focus()
	textArea.Cursor.Style = whiteStyle
	textArea.SetPromptFunc(2, func(lineIdx int) string {
		if lineIdx == 0 {
			return whiteStyle.Render("❯ ")
		}
		return "  "
	})

	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"✶", "✷", "✸", "✹", "✺", "✹", "✸", "✷"},
		FPS:    time.Second / 8,
	}
	sp.Style = systemStyle

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "?"
	}

	currentSID := ""
	currentName := ""
	cfg, _ := session.Load()
	if cfg != nil {
		currentSID = strings.TrimSpace(cfg.SessionID)
	}
	if currentSID != "" {
		if !go_pkg_filesystem_reader.IsDir(filepath.Join(filesystem.SessionsDir, currentSID)) {
			currentSID = ""
		}
	}
	if currentSID == "" {
		newID, err := session.CreateSession("cli-")
		if err != nil {
			slog.Warn("session.CreateSession",
				slog.String("error", err.Error()))
		} else {
			currentSID = newID
			if cfg == nil {
				cfg = &session.Config{}
			}
			cfg.SessionID = newID
			if err := session.Save(cfg); err != nil {
				slog.Warn("session.Save",
					slog.String("error", err.Error()))
			}
		}
	}
	if currentSID != "" {
		currentName, _ = session.GetBot(currentSID)
	}

	return TUI{
		ctx:                ctx,
		textarea:           textArea,
		spinner:            sp,
		cwd:                cwd,
		daemonStatus:       getDaemonStatus(),
		httpStatus:         getHttpStatus(),
		discordStatus:      getDiscordStatus(),
		telegramStatus:     getTelegramStatus(),
		mode:               cliMode,
		width:              80,
		currentSessionID:   currentSID,
		currentSessionName: currentName,
		inputHistory:       loadInputHistory(currentSID),
		inputHistoryIdx:    -1,
	}
}

func getDiscordStatus() string {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.DiscordEnabled || keychain.Get(discord.Key) == "" {
		return textStyle.Render("discord:  ") + hintStyle.Render("disable")
	}
	name := cfg.DiscordUsername
	if name == "" {
		name = "enabled"
	}
	return textStyle.Render("discord:  ") + okayStyle.Render(name)
}

func getTelegramStatus() string {
	cfg, err := session.Load()
	if err != nil || cfg == nil || !cfg.TelegramEnabled || keychain.Get(telegram.Key) == "" {
		return textStyle.Render("telegram: ") + hintStyle.Render("disable")
	}
	name := cfg.TelegramUsername
	if name == "" {
		name = "enabled"
	}
	return textStyle.Render("telegram: ") + okayStyle.Render(name)
}

func getDaemonStatus() string {
	r, err := runtime.Read()
	if err != nil || r == nil || !runtime.IsAlive(r.PID) {
		return textStyle.Render("daemon:   ") + errorStyle.Render("failed")
	}
	return textStyle.Render("daemon:   ") + okayStyle.Render(strconv.Itoa(r.PID))
}

func getHttpStatus() string {
	port := go_pkg_utils.GetWithDefault("PORT", "17989")
	r, err := runtime.Read()
	if err != nil || r == nil || !runtime.IsAlive(r.PID) {
		return textStyle.Render("http:     ") + errorStyle.Render("failed")
	}
	return textStyle.Render("http:     ") + okayStyle.Render(port)
}

func loadSessionTail(sid string) []tea.Cmd {
	if strings.TrimSpace(sid) == "" {
		return nil
	}
	lines := readAllLines(filepath.Join(filesystem.SessionsDir, sid, "action.log"))
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > historyLoad {
		lines = lines[len(lines)-historyLoad:]
	}

	cmds := make([]tea.Cmd, 0, len(lines)+1)
	cmds = append(cmds, tea.Println(hintStyle.Render("⎯ recent history ("+strconv.Itoa(len(lines))+")")+"\n"))
	for _, line := range lines {
		cmds = append(cmds, tea.Println(line))
	}
	return cmds
}

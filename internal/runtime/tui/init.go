package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot/discord"
	"github.com/pardnchiu/agenvoy/internal/runtime/chatbot/telegram"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	historyLoad = 100
)

type TUI struct {
	ctx      context.Context
	textarea textarea.Model
	spinner  spinner.Model

	running      bool
	emitted      bool
	cancelExec   context.CancelCauseFunc
	interruptAt  time.Time
	runStartedAt time.Time
	lastInput    string
	connecting   bool

	popup         *Popup
	popupQueue    []Pending
	eventQueue    []agenttypes.Event
	popupOrigin   *Popup
	botBodyDraft  string
	roleBodyDraft string
	mcpAdd        *mcpAddDraft
	mcpClient     *mcpClientDraft
	mcpOAuth      *oauthState
	modelAdd      *modelAddItem

	selector     *CmdSelector
	notice       string
	noticeOffset int

	currentModel       string
	quotaModel         string
	quotaText          string
	currentSessionID   string
	currentSessionName string
	activity           string
	toolLog            []string
	lastIn             int
	lastOut            int
	lastCacheRead      int
	lastCacheCreate    int
	lastContext        int

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
	tableBuf       []string

	toolBuf         []string
	toolCount       int
	subCount        int
	subActive       int
	subBuf          map[string]*subagentLive
	subOrder        []string
	todos           []agenttypes.TodoItem
	pendingSteer    []string
	inputHistory    []string
	inputHistoryIdx int

	quitting bool

	popupOnScreen bool

	allowAll bool
}

func (t TUI) Init() tea.Cmd {
	sid := strings.TrimSpace(t.currentSessionID)
	seq := []tea.Cmd{tea.ClearScreen}
	if sid != "" {
		seq = append(seq, tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)))
	}
	seq = append(seq, func() tea.Msg { return initTailer{} })
	if sid != "" {
		if n := len(interactive.ListResumablePending(sid)); n > 0 {
			hint := fmt.Sprintf("  %d pending task(s) — /pending to resume", n)
			seq = append(seq, notice(msgLog(hint)+"\n"))
		}
	} else {
		seq = append(seq, func() tea.Msg { return StartupSelectSession{} })
	}
	return tea.Batch(tea.Sequence(seq...), t.spinner.Tick)
}

type CancelRunConfirm struct {
	yes   bool
	pause bool
}

type StartupSelectSession struct{}

type StartupSessionSelect struct {
	id string
}

type StartupSessionSkip struct{}

func newModel(ctx context.Context) TUI {
	textArea := textarea.New()
	textArea.Placeholder = `/ commands  shift+u usage  shift+f fast  shift+g guide  shift+p workdir`
	textArea.CharLimit = 8000
	textArea.SetHeight(1)
	textArea.ShowLineNumbers = false
	textArea.FocusedStyle.CursorLine = lipgloss.NewStyle()
	textArea.Focus()
	textArea.Cursor.Style = whiteStyle
	textArea.Cursor.SetMode(cursor.CursorStatic)
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

	refreshBotNames()

	currentSID := ""
	currentName := ""

	sessions := listSessions()
	if len(sessions) == 1 {
		currentSID = sessions[0].id
	}
	if currentSID != "" {
		currentName, _ = configbot.Get(currentSID)
	}

	return TUI{
		ctx:                ctx,
		textarea:           textArea,
		spinner:            sp,
		cwd:                cwd,
		daemonStatus:       getDaemonStatus(),
		connecting:         true,
		httpStatus:         getHttpStatus(),
		discordStatus:      getDiscordStatus(),
		telegramStatus:     getTelegramStatus(),
		width:              80,
		currentSessionID:   currentSID,
		currentSessionName: currentName,
		inputHistory:       loadInputHistory(currentSID),
		inputHistoryIdx:    -1,
	}
}

func getDiscordStatus() string {
	cfg, err := config.Load()
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
	cfg, err := config.Load()
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
		return textStyle.Render("uid:  ") + errorStyle.Render("failed")
	}
	return textStyle.Render("uid:  ") + okayStyle.Render(strconv.Itoa(r.PID))
}

func getHttpStatus() string {
	r, err := runtime.Read()
	if err != nil || r == nil || !runtime.IsAlive(r.PID) {
		return textStyle.Render("http: ") + errorStyle.Render("failed")
	}
	return textStyle.Render("http: ") + okayStyle.Render(filesystem.Port)
}

func refreshBotNames() {
	dirs, err := go_pkg_filesystem_reader.ListDirs(filesystem.SessionsDir)
	if err != nil {
		return
	}
	for _, d := range dirs {
		refreshBotName(d.Name)
	}
}

func refreshBotName(sid string) {
	chatID, _, channelID, _ := configbot.GetChannel(sid)

	var authPath, id string
	switch {
	case strings.HasPrefix(sid, "tg-"):
		authPath, id = filesystem.TelegramAuthPath, chatID
	case strings.HasPrefix(sid, "dc-"):
		authPath, id = filesystem.DiscordAuthPath, channelID
	default:
		return
	}
	if id == "" {
		return
	}

	if n := configbot.FormatName(utils.LookupChatName(authPath, id)); n != "" {
		configbot.ReplaceDefault(sid, n)
	}
}

func loadSessionTail(sid string, width int, all bool) []tea.Cmd {
	if strings.TrimSpace(sid) == "" {
		return nil
	}
	lines := readAllLines(filesystem.ActionLogPath(sid), width)
	if len(lines) == 0 {
		return nil
	}
	label := "recent history"
	if all {
		label = "full history"
	} else if len(lines) > historyLoad {
		lines = lines[len(lines)-historyLoad:]
	}

	cmds := make([]tea.Cmd, 0, len(lines)*2+2)
	cmds = append(cmds, notice(msgLog(""+label+" ("+strconv.Itoa(len(lines))+")")+"\n"))
	for i, l := range lines {
		if i > 0 && l.kind != "done" && l.kind != "canceled" {
			cmds = append(cmds, tea.Println(""))
		}
		cmds = append(cmds, tea.Println(l.line))
	}
	cmds = append(cmds, tea.Println(""))
	return cmds
}

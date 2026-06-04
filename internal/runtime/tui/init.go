package tui

import (
	"context"
	"os"
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
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
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

	running       bool
	cancelExec    context.CancelFunc
	runStartedAt  time.Time
	pendingResume *ResumeExec

	popup        *Popup
	popupQueue   []Pending
	botBodyDraft string
	mcpAdd       *mcpAddDraft
	modelAdd     *modelAddItem

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

	onceCall     bool
	userInput    string
	allowAll     bool
	awaitingExit bool
}

func (t TUI) Init() tea.Cmd {
	var seq []tea.Cmd
	if t.onceCall {
		// agen cli/run: keep the user's terminal scrollback intact, skip the
		// header block (daemon/http/discord/telegram status), skip the
		// action.log tailer, and skip the textarea.Blink (no visible
		// textarea). Session selection mirrors `agen`: if currentSID is
		// already set (1 existing session), auto-submit straight away; else
		// fire StartupSelectSession popup and let the SessionSelect /
		// SessionNewSubmit handlers chain the autoSubmit after the user
		// picks. Skip LoadHistoryCheck (we don't render history tail in
		// single-shot output).
		if sid := strings.TrimSpace(t.currentSessionID); sid != "" {
			if input := strings.TrimSpace(t.userInput); input != "" {
				seq = append(seq, func() tea.Msg { return autoSubmit{input: input} })
			}
		} else {
			seq = append(seq, func() tea.Msg { return StartupSelectSession{} })
		}
		return tea.Sequence(seq...)
	}
	seq = []tea.Cmd{
		tea.ClearScreen,
		tea.Batch(
			textarea.Blink,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		),
	}
	seq = append(seq, func() tea.Msg { return initTailer{} })
	if sid := strings.TrimSpace(t.currentSessionID); sid != "" {
		path := filesystem.ActionLogPath(sid)
		if go_pkg_filesystem_reader.Exists(path) && fileSize(path) > 0 {
			seq = append(seq, func() tea.Msg { return LoadHistoryCheck{id: sid} })
		}
	} else {
		seq = append(seq, func() tea.Msg { return StartupSelectSession{} })
	}
	return tea.Sequence(seq...)
}

type autoSubmit struct {
	input string
}

// chainSingleShotSubmit appends an autoSubmit emit to the cmd returned by a
// session-pick handler. Single-shot suppresses the picker's own ClearScreen +
// header reprint (so `prior` is usually nil), but we still tea.Sequence to be
// safe in case future picker paths add silent housekeeping cmds.
func chainSingleShotSubmit(prior tea.Cmd, input string) tea.Cmd {
	submit := func() tea.Msg { return autoSubmit{input: strings.TrimSpace(input)} }
	if prior == nil {
		return submit
	}
	return tea.Sequence(prior, submit)
}

type StartupSelectSession struct{}

type StartupSessionSelect struct {
	id string
}

type LoadHistoryCheck struct {
	id string
}

type LoadHistorySelect struct {
	id   string
	load bool
}



func newModel(ctx context.Context, userInput string, onceCall, allowAll bool) TUI {
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

	refreshBotNames()

	currentSID := ""
	currentName := ""

	sessions := listSessions()
	if len(sessions) == 1 {
		currentSID = sessions[0].id
	}
	if currentSID != "" {
		currentName, _ = configBot.Get(currentSID)
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
		onceCall:           onceCall,
		userInput:          userInput,
		allowAll:           allowAll,
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
		return textStyle.Render("daemon:   ") + errorStyle.Render("failed")
	}
	return textStyle.Render("daemon:   ") + okayStyle.Render(strconv.Itoa(r.PID))
}

func getHttpStatus() string {
	r, err := runtime.Read()
	if err != nil || r == nil || !runtime.IsAlive(r.PID) {
		return textStyle.Render("http:     ") + errorStyle.Render("failed")
	}
	return textStyle.Render("http:     ") + okayStyle.Render(filesystem.Port)
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
	var authPath, idKey string
	switch {
	case strings.HasPrefix(sid, "tg-"):
		authPath = filesystem.TelegramAuthPath
		idKey = "chat_id"
	case strings.HasPrefix(sid, "dc-"):
		authPath = filesystem.DiscordAuthPath
		idKey = "channel_id"
	default:
		return
	}
	cfg, err := go_pkg_filesystem.ReadJSON[map[string]string](filesystem.SessionConfigPath(sid))
	if err != nil {
		return
	}
	id := cfg[idKey]
	if id == "" {
		return
	}
	if n := configBot.FormatName(utils.LookupChatName(authPath, id)); n != "" {
		configBot.ReplaceDefault(sid, n)
	}
}

func loadSessionTail(sid string) []tea.Cmd {
	if strings.TrimSpace(sid) == "" {
		return nil
	}
	lines := readAllLines(filesystem.ActionLogPath(sid))
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

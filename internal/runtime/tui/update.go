package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/internal/agents"
	execchatbot "github.com/pardnchiu/agenvoy/internal/agents/exec/chatbot"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/guide"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/ipc"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type popupScreenReady struct{}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if state, ok := msg.(daemonState); ok {
		t.connecting = !state.connected
		if t.connecting {
			return t, t.spinner.Tick
		}
		return t, nil
	}
	if _, ok := msg.(popupScreenReady); ok {
		t.popupOnScreen = t.popup != nil
		return t, nil
	}
	wasOpen := t.popup != nil
	next, cmd := t.update(msg)
	nt, ok := next.(TUI)
	if !ok {
		return next, cmd
	}
	switch isOpen := nt.popup != nil; {
	case !wasOpen && isOpen:
		nt.popupOnScreen = false
		return nt, tea.Batch(tea.Sequence(tea.EnterAltScreen, func() tea.Msg { return popupScreenReady{} }), cmd)
	case wasOpen && !isOpen:
		nt.popupOnScreen = false
		cmds := []tea.Cmd{tea.ExitAltScreen, cmd}
		queued := nt.eventQueue
		nt.eventQueue = nil
		for _, ev := range queued {
			model, evCmd := nt.handleAgentEvent(ev)
			nt = model.(TUI)
			cmds = append(cmds, evCmd)
		}
		return nt, tea.Sequence(cmds...)
	}
	return nt, cmd
}

func (t TUI) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if t.popup != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			return t.updatePopup(msg)
		case spinner.TickMsg:
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
			return t, tea.Batch(cmds...)
		case Pending:
			t.popupQueue = append(t.popupQueue, msg)
			return t, nil
		case agentEvent:
			t.eventQueue = append(t.eventQueue, msg.event)
			return t, nil
		case OAuthInfo:
			return t.runOAuthInfo(msg)
		case OAuthSuccess:
			return t.runOAuthSuccess(msg)
		case OAuthFailed:
			return t.runOAuthFailed(msg)
		case McpOAuthInfo:
			return t.runMcpOAuthInfo(msg)
		case McpOAuthDone:
			return t.runMcpOAuthDone(msg)
		case McpOAuthPaste:
			return t.runMcpOAuthPaste(msg)
		case RoutingTabLoaded:
			return t.runRoutingTabLoaded(msg)
		case CompatModelsResult:
			return t.runCompatModelsResult(msg)
		case RemoteModelsResult:
			return t.runRemoteModelsResult(msg)
		case Log:
			return t, notice(renderLogLine(msg))
		case noticeMsg:
			t.notice = appendNotice(t.notice, msg.text)
			t.noticeOffset = 0
			return t, nil
		case QuotaLoaded:
			t.quotaModel, t.quotaText = msg.model, msg.text
			return t, nil
		}
		return t, nil
	}

	switch msg := msg.(type) {
	case popupChild:
		return t.runPopupChild(msg)

	case QuotaLoaded:
		t.quotaModel, t.quotaText = msg.model, msg.text
		return t, nil

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.textarea.SetWidth(msg.Width - 4)
		return t, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return t.handleInterrupt()

		case tea.KeyEsc:
			if t.selector != nil {
				t.selector = nil
				return t, nil
			}
			if t.notice != "" {
				t.notice, t.noticeOffset = "", 0
				return t, nil
			}
			if t.running && t.cancelExec != nil {
				if !t.emitted {
					return t.update(CancelRunConfirm{yes: true})
				}
				t.popup = &Popup{
					kind:    popupSingleSelect,
					title:   "Cancel current task?",
					options: []string{"No", "Yes    cancel", "Pause  keep pending"},
					values:  []string{"no", "yes", "pause"},
					onConfirm: func(chosen string) any {
						return CancelRunConfirm{yes: chosen == "yes", pause: chosen == "pause"}
					},
				}
				return t, nil
			}
			if t.textarea.Value() != "" {
				t.textarea.Reset()
				t.textarea.SetHeight(1)
				t.inputHistoryIdx = -1
				return t, nil
			}

		case tea.KeyRunes:
			if !t.running && t.selector == nil && strings.TrimSpace(t.textarea.Value()) == "" {
				switch string(msg.Runes) {
				case "W":
					return t.cycleDispatcher(false)
				case "S":
					return t.cycleDispatcher(true)
				case "A":
					return t.cycleReasoning(false)
				case "D":
					return t.cycleReasoning(true)
				case "F":
					if fast.IsEnabled() {
						fast.Disable()
					} else {
						fast.Enable()
					}
					return t, nil
				case "G":
					if guide.IsEnabled() {
						guide.Disable()
					} else {
						guide.Enable()
					}
					return t, nil
				case "U":
					next, cmd, _ := t.commandProviderUsage()
					return next, cmd
				case "P":
					return t.commandWorkDir(), nil
				}
			}

		case tea.KeyUp:
			if t.selector != nil {
				n := len(t.selector.items)
				t.selector.cursor = (t.selector.cursor - 1 + n) % n
				return t, nil
			}

			if !t.running && (t.inputHistoryIdx >= 0 || t.textarea.Line() == 0) {
				if next, handled := t.clickUp(); handled {
					return next, nil
				}
			}

		case tea.KeyDown:
			if t.selector != nil {
				n := len(t.selector.items)
				t.selector.cursor = (t.selector.cursor + 1) % n
				return t, nil
			}

			if !t.running && (t.inputHistoryIdx >= 0 || t.textarea.Line() == t.textarea.LineCount()-1) {
				if next, handled := t.clickDown(); handled {
					return next, nil
				}
			}

		case tea.KeyShiftTab:
			if t.selector != nil || t.running {
				return t, nil
			}
			t.allowAll = !t.allowAll
			return t, nil

		case tea.KeyTab:
			if t.selector != nil {
				t = t.selectCommand()
				return t, nil
			}
			if t.notice != "" {
				return t.scrollNotice(), nil
			}

		case tea.KeyEnter:
			if t.selector != nil {
				t = t.selectCommand()
				return t, nil
			}

			if msg.Alt {
				t.textarea.InsertRune('\n')
				t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
				return t, nil
			}

			if t.running {
				raw := t.textarea.Value()
				if trimmed := strings.TrimSpace(raw); trimmed == "" || strings.HasPrefix(trimmed, "/") {
					return t, nil
				}
				if err := ipcClient.Load().Steer(t.currentSessionID, sessiontui.Get(), raw); err != nil {
					return t, notice(msgError(fmt.Sprintf("steer: %v", err)) + "\n")
				}
				t.pendingSteer = append(t.pendingSteer, raw)
				t.textarea.Reset()
				t.textarea.SetHeight(1)
				return t, nil
			}

			raw := t.textarea.Value()
			content := strings.TrimSpace(raw)
			if content == "" {
				return t, nil
			}
			if t.connecting && (!strings.HasPrefix(content, "/") || noMatches(content)) {
				return t, nil
			}
			t = t.recordInputHistory(raw)
			t.lastInput = raw
			t.textarea.Reset()
			t.textarea.SetHeight(1)

			if strings.HasPrefix(content, "/") {
				if next, cmd, handled := t.handleCommand(content); handled {
					return next, cmd
				}
				if !noMatches(content) {
					return t, notice(msgWarn(fmt.Sprintf("unknown command: %s", strings.Fields(content)[0])) + "\n")
				}
			}

			if len(agents.ModelSet().Order) == 0 {
				return t, notice(msgWarn("no model configured  /model global add") + "\n")
			}

			t.running = true
			t.runStartedAt = time.Now()
			t.activity = ""
			t.currentModel = ""
			t.lastIn, t.lastOut, t.lastCacheRead, t.lastCacheCreate, t.lastContext = 0, 0, 0, 0, 0
			t.runTarget = ""

			if command, ok := strings.CutPrefix(content, "!"); ok {
				command = strings.TrimSpace(command)
				if command == "" {
					t.running = false
					return t, notice(msgWarn("empty command after !") + "\n")
				}
				go runShell(command, t.cwd)
				return t, tea.Batch(tea.Println(messageBlock(raw)), t.spinner.Tick)
			}

			go runExec(ipc.Frame{Type: ipc.FrameRun, SessionID: t.currentSessionID, Rayload: &ipc.Payload{Input: raw, WorkDir: t.cwd, AllowAll: t.allowAll}})

			cmds = append(cmds,
				tea.Println(messageBlock(raw)),
				t.spinner.Tick,
			)
			return t, tea.Batch(cmds...)
		}

	case agentExec:
		t.cancelExec = msg.cancel
		t.emitted = false
		t.toolBuf, t.toolCount = nil, 0
		t.subCount, t.subActive = 0, 0
		t.subBuf, t.subOrder = nil, nil
		return t, nil

	case shellDone:
		return t.handleShellDone(msg)

	case agentExecDone:
		t.running = false
		t.cancelExec = nil
		t.activity = ""
		t.runTarget = ""
		t.streaming = false
		leftoverSteer := t.pendingSteer
		t.pendingSteer = nil
		if errors.Is(msg.err, context.Canceled) {
			restore := t.lastInput
			if len(leftoverSteer) > 0 {
				if restore != "" {
					restore += "\n"
				}
				restore += strings.Join(leftoverSteer, "\n")
			}
			if restore != "" {
				t.textarea.SetValue(restore)
				t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
				t.lastInput = ""
			}
		}
		if t.currentSessionID != "" {
			t.currentSessionName, _ = configbot.Get(t.currentSessionID)
		}
		var doneCmds []tea.Cmd
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			doneCmds = append(doneCmds, notice(msgError(fmt.Sprintf("exec error: %v", msg.err))+"\n"))
		}
		if len(doneCmds) == 0 {
			return t, nil
		}
		return t, tea.Sequence(doneCmds...)

	case PendingSelect:
		return t.resumePending(msg)

	case agentEvent:
		return t.handleAgentEvent(msg.event)

	case Pending:
		popup := newPopup(msg)
		if popup == nil {
			resolvePending(msg.id, runtime.Reply{Error: utilsmsg.New(utilsmsg.Invalid, "pending request", "no popup for this request")})
			return t, nil
		}

		if msg.request.Kind == runtime.KindAskUser && !msg.request.Inline {
			if t.cancelExec != nil {
				t.cancelExec(nil)
				t.cancelExec = nil
			}
			t.running = false
			t.activity = ""
			t.runTarget = ""
			t.streaming = false
		}
		t.popup = popup
		return t, nil

	case SessionSelect:
		return t.runCommandSwitch(msg.id)

	case SessionNewCustomSubmit:
		next, cmd := t.showNewCustomPopup(msg.name)
		return next, cmd

	case SessionNewPromptSubmit:
		return t.runCreateSession(msg.name, msg.body)

	case SessionModelSelect:
		return t.runSessionModelSelect(msg.name)

	case SessionReasoningSelect:
		return t.runSessionReasoningSelect(msg.level)

	case ModelOrderSave:
		return t.runModelOrderSave(msg)

	case ModelScopeSelect:
		switch msg.scope {
		case "add":
			next, cmd, _ := t.commandModelAdd()
			return next, cmd
		}
		return t, nil

	case McpMenuPick:
		return t.runMcpMenuPick(msg.value)

	case McpServerAction:
		return t.runMcpServerAction(msg)

	case McpRemovePick:
		return t.openMcpRemoveConfirm(msg.server)

	case McpRemoveConfirm:
		if !msg.yes {
			next, cmd, _ := t.commandMcp(nil)
			return next, cmd
		}
		next, cmd := t.runMcpRemove(msg.server)
		next, _, _ = next.commandMcp(nil)
		return next, cmd

	case McpReconnectDone:
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("%s reconnect: %v", msg.server, msg.err)) + "\n")
		}
		slog.Debug("mcp reconnected", slog.String("server", msg.server))
		return t, nil

	case McpPermissionResult:
		return t.runMcpPermissionResult(msg)

	case McpPermissionPick:
		return t.runMcpPermissionPick(msg)

	case SkillsInstallPick:
		return t.runSkillsInstallPick(msg)

	case SkillsInstallDone:
		return t.runSkillsInstallDone(msg)

	case SkillSourcePick:
		return t.runSkillSourcePick(msg)

	case McpOAuthPaste:
		return t.runMcpOAuthPaste(msg)

	case McpAddName:
		if msg.name == "" {
			t.mcpAdd = nil
			return t, notice(msgError("mcp name required") + "\n")
		}
		if !isValidMcpServerName(msg.name) {
			t.mcpAdd = nil
			return t, notice(msgError("mcp name must match [A-Za-z0-9_-]") + "\n")
		}
		t.mcpAdd.name = msg.name
		next, cmd := t.openMcpAddTransport()
		return next, cmd

	case McpAddTransport:
		t.mcpAdd.transport = msg.transport
		switch msg.transport {
		case "stdio":
			next, cmd := t.openMcpAddCommand()
			return next, cmd
		case "streamable-http":
			next, cmd := t.openMcpAddURL()
			return next, cmd
		}
		t.mcpAdd = nil
		return t, nil

	case McpAddCommand:
		if msg.command == "" {
			t.mcpAdd = nil
			return t, notice(msgError("command required") + "\n")
		}
		t.mcpAdd.command = msg.command
		next, cmd := t.openMcpAddArgs()
		return next, cmd

	case McpAddArgs:
		t.mcpAdd.args = parseArgsCSV(msg.raw)
		next, cmd := t.openMcpAddEnv()
		return next, cmd

	case McpAddEnv:
		t.mcpAdd.env = parseKV(msg.raw)
		return t.finalizeMcpAdd()

	case McpAddURL:
		if msg.url == "" {
			t.mcpAdd = nil
			return t, notice(msgError("url required") + "\n")
		}
		t.mcpAdd.url = msg.url
		next, cmd := t.openMcpAddAuthMethod()
		return next, cmd

	case McpAddAuthMethod:
		switch msg.method {
		case "none":
			return t.openMcpAddExtraHeaders()
		case "oauth":
			t.mcpAdd.auth = "oauth"
			return t.openMcpAddExtraHeaders()
		case "bearer":
			return t.openMcpAddBearerToken()
		case "apikey":
			return t.openMcpAddAPIKeyHeader()
		case "basic":
			return t.openMcpAddBasicToken()
		}
		t.mcpAdd = nil
		return t, nil

	case McpAddBearerToken:
		t.mcpAdd.headers = bearerAuthorizationHeader(msg.token)
		return t.openMcpAddExtraHeaders()

	case McpAddAPIKeyHeader:
		t.mcpAdd.authHeaderName = msg.header
		if t.mcpAdd.authHeaderName == "" {
			t.mcpAdd.authHeaderName = "X-API-Key"
		}
		return t.openMcpAddAPIKeyValue(t.mcpAdd.authHeaderName)

	case McpAddAPIKeyValue:
		t.mcpAdd.headers = apiKeyHeader(t.mcpAdd.authHeaderName, msg.value)
		return t.openMcpAddExtraHeaders()

	case McpAddBasicToken:
		t.mcpAdd.headers = basicAuthorizationHeader(msg.token)
		return t.openMcpAddExtraHeaders()

	case McpAddExtraHeaders:
		if !msg.yes {
			return t.finalizeMcpAdd()
		}
		return t.openMcpAddHeaders()

	case McpAddHeaders:
		t.mcpAdd.headers = mergeMcpHeaders(t.mcpAdd.headers, parseKV(msg.raw))
		return t.finalizeMcpAdd()

	case McpAddSaved:
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("mcp add: %v", msg.err)) + "\n")
		}
		if msg.oauth {
			slog.Debug("mcp added", slog.String("server", msg.name))
			return t.startMcpLogin(msg.name)
		}
		slog.Debug("mcp added", slog.String("server", msg.name))
		return t.reconnectMcpServer(msg.name)

	case McpClientID:
		if msg.id == "" {
			t.mcpClient = nil
			return t, notice(msgError("client id required") + "\n")
		}
		t.mcpClient.id = msg.id
		return t.openMcpClientSecret()

	case McpClientSecret:
		t.mcpClient.secret = msg.secret
		return t.openMcpClientRedirect()

	case McpClientRedirect:
		return t.runMcpClientSave(msg.uri)

	case McpOAuthInfo:
		return t.runMcpOAuthInfo(msg)

	case McpOAuthDone:
		return t.runMcpOAuthDone(msg)

	case ModelTagPick:
		return t.openModelTagPicker(msg.name)

	case ModelTagSubmit:
		if err := config.SetModelTag(msg.name, msg.tag); err != nil {
			return t, notice(msgError(fmt.Sprintf("model tag: %v", err)) + "\n")
		}
		next, _, _ := t.commandModel(nil)
		return next, nil

	case ModelRemovePick:
		next, cmd := t.openModelRemoveConfirm(msg.name)
		return next, cmd

	case ModelRemoveConfirm:
		if !msg.yes {
			next, cmd, _ := t.commandModel(nil)
			return next, cmd
		}
		next, cmd := t.runModelRemove(msg.name)
		next, _, _ = next.commandModel(nil)
		return next, cmd

	case BotFieldPick:
		sid := strings.TrimSpace(t.currentSessionID)
		if sid == "" {
			return t, notice(msgError("no current session") + "\n")
		}
		next, cmd := t.openBotField(sid, msg.field)
		return next, cmd

	case BotFieldSubmit:
		return t.runBotFieldSubmit(msg)

	case BotCustomSubmit:
		next, cmd := t.showBotCustomPopup(msg.name, msg.selfID)
		return next, cmd

	case BotPromptSubmit:
		sid := strings.TrimSpace(t.currentSessionID)
		if sid == "" {
			return t, notice(msgError("no current session") + "\n")
		}
		return t, t.botSaveCmd(sid, msg.selfID, msg.name, msg.body)

	case RoleListed:
		return t.runRoleListed(msg)

	case RolePick:
		return t.runRolePick(msg)

	case RoleLoaded:
		return t.runRoleLoaded(msg)

	case RoleTitleSubmit:
		return t.runRoleTitleSubmit(msg)

	case RoleBodySubmit:
		return t, t.roleSaveCmd(msg)

	case RoleSaved:
		return t.runRoleSaved(msg)

	case BotSaved:
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("bot save: %v", msg.err)) + "\n")
		}
		if t.currentSessionID != "" {
			t.currentSessionName = msg.name
		}
		slog.Debug("bot saved", slog.String("name", msg.name))
		return t, nil

	case ModelAddProviderPick:
		return t.runModelAddProviderPick(msg.provider)

	case ModelAddAPIKeyReplace:
		return t.runModelAddAPIKeyReplace(msg.replace)

	case ModelAddAPIKeySubmit:
		return t.runModelAddAPIKeySubmit(msg.key)

	case ModelAddVectorNotice:
		return t.openModelAddModelPick()

	case ModelAddAccountIDReplace:
		return t.runModelAddAccountIDReplace(msg.replace)

	case ModelAddAccountIDSubmit:
		return t.runModelAddAccountIDSubmit(msg.id)

	case ModelAddGatewayIDPick:
		return t.runModelAddGatewayIDPick(msg.chosen)

	case ModelAddGatewayIDSubmit:
		return t.runModelAddGatewayIDSubmit(msg.id)

	case ModelAddCompatNameSubmit:
		return t.runModelAddCompatNameSubmit(msg.name)

	case ModelAddCompatURLSubmit:
		return t.runModelAddCompatURLSubmit(msg.url)

	case ModelAddCompatKeySubmit:
		return t.runModelAddCompatKeySubmit(msg.key)

	case ModelAddModelMultiPick:
		return t.runModelAddModelMultiPick(msg.chosen)

	case CompatModelsResult:
		return t.runCompatModelsResult(msg)

	case RemoteModelsResult:
		return t.runRemoteModelsResult(msg)

	case ProviderUsageResult:
		return t, notice(msgLog("") + " " + strings.Join(msg.lines, " / ") + "\n")

	case OAuthInfo:
		return t.runOAuthInfo(msg)

	case OAuthSuccess:
		return t.runOAuthSuccess(msg)

	case OAuthFailed:
		return t.runOAuthFailed(msg)

	case OAuthReLoginPick:
		return t.runOAuthReLoginPick(msg.replace)

	case ModelAddDone:
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
		}
		if msg.err != nil {
			seq = append(seq, notice(msgError(fmt.Sprintf("add-model: %v", msg.err))+"\n"))
		} else {
			agents.Reload()
			slog.Debug("model added")
		}
		return t, tea.Sequence(seq...)

	case DiscordAction:
		switch msg.action {
		case "enable":
			next, cmd := t.openDiscordTokenPrompt()
			return next, cmd
		case "disable":
			return t, tea.Sequence(
				notice(msgLog("discord disabling")+"\n"),
				disableDiscord(),
			)
		}
		return t, nil

	case DiscordTokenSubmit:
		return t, tea.Sequence(
			notice(msgLog("discord verifying token (≤10s)")+"\n"),
			enableDiscord(msg.token),
		)

	case TelegramAction:
		switch msg.action {
		case "enable":
			next, cmd := t.openTelegramTokenPrompt()
			return next, cmd
		case "disable":
			return t, tea.Sequence(
				notice(msgLog("telegram disabling")+"\n"),
				disableTelegram(),
			)
		}
		return t, nil

	case TelegramTokenSubmit:
		return t, tea.Sequence(
			notice(msgLog("telegram verifying token (≤10s)")+"\n"),
			enableTelegram(msg.token),
		)

	case ScheduleTestPick:
		next, cmd := t.runScheduleTest(msg.skill)
		return next, cmd

	case ScheduleRemovePick:
		next, cmd := t.openScheduleRemoveConfirm(msg.kind, msg.skill)
		return next, cmd

	case ScheduleRemoveConfirm:
		if !msg.yes {
			next, cmd := t.reopenSchedule()
			return next, cmd
		}
		next, cmd := t.runScheduleRemove(msg.kind, msg.skill)
		return next, cmd

	case CancelRunConfirm:
		if t.cancelExec == nil {
			return t, nil
		}
		switch {
		case msg.yes:
			t.cancelExec(runtime.ErrUserCanceled)
		case msg.pause:
			t.cancelExec(nil)
		}
		return t, nil

	case PendingDeletePick:
		next, cmd := t.openPendingDeleteConfirm(msg)
		return next, cmd

	case PendingDeleteConfirm:
		if !msg.yes {
			next, cmd, _ := t.commandPending()
			return next, cmd
		}
		next, cmd := t.runPendingDelete(msg)
		return next, cmd

	case SessionDeletePick:
		next, cmd := t.openSessionDeleteConfirm(msg.id)
		return next, cmd

	case RemoveSessionConfirm:
		return t.runRemoveSessionConfirm(msg)

	case ResetSessionConfirm1:
		if msg.mode != "summary" && msg.mode != "all" {
			return t, nil
		}
		next, cmd := t.openResetConfirm2(msg.id, msg.mode)
		return next, cmd

	case ResetSessionConfirm2:
		if !msg.yes {
			return t, nil
		}
		next, cmd := t.runResetSession(msg.id, msg.mode)
		return next, cmd

	case ResetSessionDone:
		next, cmd := t.finishResetSession(msg)
		return next, cmd

	case CompactConfirm:
		if !msg.yes {
			return t, nil
		}
		return t.runCompact(msg.id)

	case CompactDone:
		return t.finishCompact(msg)

	case DiscordDone:
		t.discordStatus = getDiscordStatus()
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
		}
		if msg.err != nil {
			seq = append(seq, notice(msgError(fmt.Sprintf("discord %s: %v", msg.action, msg.err))+"\n"))
		} else {
			slog.Debug("discord toggled", slog.String("action", msg.action))
		}
		return t, tea.Sequence(seq...)

	case TelegramDone:
		t.telegramStatus = getTelegramStatus()
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
		}
		if msg.err != nil {
			seq = append(seq, notice(msgError(fmt.Sprintf("telegram %s: %v", msg.action, msg.err))+"\n"))
		} else {
			slog.Debug("telegram toggled", slog.String("action", msg.action))
		}
		return t, tea.Sequence(seq...)

	case ConfigSelect:
		return t.runConfigSelect(msg.key)

	case StartupDone:
		t = t.openConfig(configStartup)
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("startup %s: %v", msg.action, msg.err)) + "\n")
		}
		return t, nil

	case ChannelRevokePick:
		next, cmd := t.openChannelRevokeConfirm(msg)
		return next, cmd

	case ChannelRevokeConfirm:
		if !msg.yes {
			next, cmd, _ := t.commandChannel([]string{"channel", msg.channel})
			return next, cmd
		}
		return t, revokeChannelChat(msg.channel, msg.id, msg.label)

	case ChannelRevokeDone:
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("revoke %s: %v", msg.channel, msg.err)) + "\n")
		}
		next, cmd, _ := t.commandChannel([]string{"channel", msg.channel})
		slog.Debug("channel revoked", slog.String("name", msg.name))
		return next, cmd

	case KeyDeletePick:
		next, cmd := t.openKeyDeleteConfirm(msg.key)
		return next, cmd

	case KeyDeleteConfirm:
		if !msg.yes {
			next, cmd, _ := t.commandKey(nil)
			return next, cmd
		}
		next, cmd := t.runKeyDelete(msg.key)
		return next, cmd

	case ChannelSelect:
		switch msg.channel {
		case "telegram":
			return t.openTelegramTokenPrompt()
		case "discord":
			return t.openDiscordTokenPrompt()
		}
		return t, nil

	case AdminChannelSubmit:
		value := strings.TrimSpace(msg.value)
		if value != "" {
			if _, _, ok := execchatbot.ParseAdmin(value); !ok {
				return t, notice(msgError("channel admin: format must be tg@<chatID> or dc@<channelID>") + "\n")
			}
		}
		cfg, err := config.Load()
		if err != nil || cfg == nil {
			return t, notice(msgError(fmt.Sprintf("channel admin: session.Load: %v", err)) + "\n")
		}
		cfg.AdminChannel = value
		if err := config.Save(cfg); err != nil {
			return t, notice(msgError(fmt.Sprintf("channel admin: session.Save: %v", err)) + "\n")
		}
		slog.Debug("channel admin updated", slog.String("value", value))
		return t.openConfig(configAdminChat), nil

	case KeySelect:
		next, cmd := t.openKeyValuePrompt(msg.key)
		return next, cmd

	case KeySubmit:
		if msg.value == "" {
			return t, notice(msgError(fmt.Sprintf("key %s: value is required", msg.key)) + "\n")
		}
		if err := keychain.Set(msg.key, msg.value); err != nil {
			return t, notice(msgError(fmt.Sprintf("keychain.Set %s: %v", msg.key, err)) + "\n")
		}
		slog.Debug("key updated", slog.String("key", msg.key))
		return t, nil

	case TypesafeKeySubmit:
		return t.runTypesafeKeySubmit(msg.field, msg.value)

	case DispatcherSelect:
		next, cmd := t.runDispatcherSelect(msg.name)
		agents.Reload()
		return next, cmd

	case SummaryModelSelect:
		next, cmd := t.runSummaryModelSelect(msg.name)
		agents.Reload()
		return next, cmd

	case ReplyLanguageSelect:
		return t.runReplyLanguageSelect(msg.code)

	case WorkDirSubmit:
		return t.runWorkDirSubmit(msg.value)

	case OutputDirSubmit:
		return t.runOutputDirSubmit(msg.value)

	case AudioModelSelect:
		next, cmd := t.runAudioModelSelect(msg.kind, msg.name)
		return next, cmd

	case ImageModelSelect:
		next, cmd := t.runImageModelSelect(msg.name)
		return next, cmd
	case UpdateConfirm:
		if !msg.ok {
			return t, nil
		}
		return t, tea.Sequence(
			notice(msgLog("stopping daemon  downloading latest  expect sudo prompt")+"\n"),
			runUpdateExec(),
		)

	case UpdateDone:
		t.quitting = true
		if msg.err != nil {
			return t, tea.Sequence(
				notice(msgError(fmt.Sprintf("update: %v", msg.err))+"\n"),
				tea.Quit,
			)
		}
		return t, tea.Quit

	case restrictedVerified:
		if msg.err != nil {
			return t, notice(msgError(fmt.Sprintf("restricted path: %v", msg.err)) + "\n")
		}
		return t, notice(msgWarn("⚠ restricted path approved") + "\n")

	case LogDone:
		if msg.err != nil {
			return t, tea.Sequence(
				tea.ClearScreen,
				tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
				notice(msgError(fmt.Sprintf("log: %v", msg.err))+"\n"),
			)
		}
		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
		)

	case StartupSelectSession:
		popup := popupSwitch("")
		if popup == nil {
			return t, tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID))
		}
		popup.title = "Pick session to attach"
		popup.subtitle = "Select the session this TUI will work in"
		popup.onConfirm = func(chosen string) any {
			return StartupSessionSelect{id: chosen}
		}
		popup.onCancel = func() any {
			return StartupSessionSkip{}
		}
		t.popup = popup
		return t, nil

	case StartupSessionSkip:
		return t, tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID))

	case StartupSessionSelect:
		t.currentSessionID = msg.id
		t.currentSessionName, _ = configbot.Get(msg.id)
		t.inputHistory = loadInputHistory(msg.id)
		t.inputHistoryIdx = -1
		t = t.restartTailer()
		t.tokens = 0
		t.lastIn = 0
		t.lastContext = 0
		t.lastOut = 0
		t.lastCacheRead = 0
		t.lastCacheCreate = 0
		t.currentModel = ""
		t.activity = ""

		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus, t.currentSessionID)),
		)

	case tailLine:
		return t, tea.Println(msg.line)

	case Log:
		return t, notice(renderLogLine(msg))

	case noticeMsg:
		t.notice = appendNotice(t.notice, msg.text)
		t.noticeOffset = 0
		return t, nil

	case initTailer:
		return t.restartTailer(), nil

	case spinner.TickMsg:
		if t.running || t.connecting {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if key, ok := msg.(tea.KeyMsg); ok && key.Paste {
		if quoted, ok := quoteDroppedPaths(string(key.Runes)); ok {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(quoted), Paste: true}
		}
	}

	prev := t.textarea.Value()

	var cmd tea.Cmd
	t.textarea, cmd = t.textarea.Update(msg)
	cmds = append(cmds, cmd)
	t.textarea.SetHeight(max(1, min(t.textarea.LineCount(), 5)))
	if t.textarea.Value() != prev {
		t.inputHistoryIdx = -1
		t = t.refreshCmdSelector()
	}

	return t, tea.Batch(cmds...)
}

func (t TUI) startPending(sessionID, taskHash string) (tea.Model, tea.Cmd) {
	t.running = true
	t.runStartedAt = time.Now()
	t.activity = ""
	t.currentModel = ""
	t.lastIn, t.lastOut, t.lastCacheRead, t.lastCacheCreate, t.lastContext = 0, 0, 0, 0, 0
	t.runTarget = ""
	go runExec(ipc.Frame{Type: ipc.FramePending, SessionID: sessionID, Rayload: &ipc.Payload{PendingTask: taskHash, WorkDir: t.cwd, AllowAll: t.allowAll}})
	return t, t.spinner.Tick
}

func noMatches(input string) bool {
	if scanner := agents.Scanner(); scanner != nil {
		if m, _ := runtime.MatchSkill(scanner, input); m != nil {
			return true
		}
	}
	return false
}

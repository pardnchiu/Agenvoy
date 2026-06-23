package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/kuradb"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if t.onceCall {
		if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyCtrlC {
			if t.cancelExec != nil {
				t.cancelExec()
			}
			t.quitting = true
			return t, tea.Quit
		}
	}

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
			if msg.request.Kind == runtime.KindExecProcess {
				runtime.Resolve(msg.id, runtime.Reply{
					Error: fmt.Errorf("exec process cannot run while a popup is active"),
				})
				return t, nil
			}
			t.popupQueue = append(t.popupQueue, msg)
			return t, nil
		case OAuthInfo:
			return t.runOAuthInfo(msg)
		case OAuthSuccess:
			return t.runOAuthSuccess(msg)
		case OAuthFailed:
			return t.runOAuthFailed(msg)
		}
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.textarea.SetWidth(msg.Width - 4)
		return t, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return t, nil

		case tea.KeyEsc:
			if t.selector != nil {
				t.selector = nil
				return t, nil
			}
			if t.running && t.cancelExec != nil {
				t.cancelExec()
				return t, tea.Println(hintStyle.Render("⎯ cancelling…") + "\n")
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

		case tea.KeyEnter:
			if t.awaitingExit {
				t.quitting = true
				return t, tea.Quit
			}
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
				if strings.TrimSpace(t.textarea.Value()) == "" {
					return t, nil
				}
				return t, tea.Println(hintStyle.Render("⎯ busy · esc to cancel · queue comming soon"))
			}

			content := strings.TrimSpace(t.textarea.Value())
			if content == "" {
				return t, nil
			}
			t = t.recordInputHistory(content)
			t.textarea.Reset()
			t.textarea.SetHeight(1)

			if strings.HasPrefix(content, "/") {
				if next, cmd, handled := t.handleCommand(content); handled {
					return next, cmd
				}
				if !noMatches(content) {
					return t, tea.Println(warnStyle.Render(fmt.Sprintf("⎯ unknown command: %s", strings.Fields(content)[0])) + "\n")
				}
			}

			if len(agents.Registry().Entries) == 0 {
				return t, tea.Println(warnStyle.Render("⎯ no model configured · /model global add") + "\n")
			}

			t.running = true
			t.runStartedAt = time.Now()
			t.runTarget = targetSession(content, t.currentSessionID)

			go runExec(t.ctx, content, t.allowAll, t.cwd, t.currentSessionID, "")

			cmds = append(cmds,
				tea.Println(messageBlock(content)),
				t.spinner.Tick,
			)
			return t, tea.Batch(cmds...)
		}

	case agentExec:
		t.cancelExec = msg.cancel
		return t, nil

	case agentExecDone:
		t.running = false
		t.cancelExec = nil
		t.activity = ""
		t.runTarget = ""
		t.streaming = false
		if t.currentSessionID != "" {
			t.currentSessionName, _ = configBot.Get(t.currentSessionID)
		}
		if t.pendingResume != nil {
			resume := *t.pendingResume
			t.pendingResume = nil
			return t.startResume(resume)
		}
		var doneCmds []tea.Cmd
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			doneCmds = append(doneCmds, tea.Println(errorStyle.Render(fmt.Sprintf("[!] exec error: %v", msg.err))+"\n"))
		}
		if t.onceCall {
			t.awaitingExit = true
			doneCmds = append(doneCmds, tea.Println(hintStyle.Render("⎯ press Enter to close")+"\n"))
		}
		if len(doneCmds) == 0 {
			return t, nil
		}
		return t, tea.Sequence(doneCmds...)

	case autoSubmit:
		content := strings.TrimSpace(msg.input)
		if content == "" {
			t.awaitingExit = true
			return t, tea.Println(hintStyle.Render("⎯ press Enter to close") + "\n")
		}
		if strings.HasPrefix(content, "/") {
			if next, cmd, handled := t.handleCommand(content); handled {
				return next, cmd
			}
			if !noMatches(content) {
				return t, tea.Println(warnStyle.Render(fmt.Sprintf("⎯ unknown command: %s", strings.Fields(content)[0])) + "\n")
			}
		}
		if len(agents.Registry().Entries) == 0 {
			t.awaitingExit = true
			return t, tea.Sequence(
				tea.Println(warnStyle.Render("⎯ no model configured · /model global add")+"\n"),
				tea.Println(hintStyle.Render("⎯ press Enter to close")+"\n"),
			)
		}
		t.running = true
		t.runStartedAt = time.Now()
		t.runTarget = targetSession(content, t.currentSessionID)
		go runExec(t.ctx, content, t.allowAll, t.cwd, t.currentSessionID, "")
		return t, tea.Batch(
			tea.Println(messageBlock(content)),
			t.spinner.Tick,
		)

	case ResumeExec:
		if t.running {
			t.pendingResume = &msg
			return t, nil
		}
		return t.startResume(msg)

	case PendingSelect:
		return t.resumePending(msg)

	case WorkDir:
		t.cwd = msg.dir
		return t, nil

	case agentEvent:
		return t.handleAgentEvent(msg.event)

	case Pending:
		if msg.request.Kind == runtime.KindExecProcess {
			return t.runExecProcess(msg.id, msg.request)
		}
		popup := newPopup(msg.id, msg.request)
		if popup == nil {
			runtime.Resolve(msg.id, runtime.Reply{Error: fmt.Errorf("invalid pending request")})
			return t, nil
		}

		if msg.request.Kind == runtime.KindAskUser {
			if t.cancelExec != nil {
				t.cancelExec()
				t.cancelExec = nil
			}
			t.running = false
			t.activity = ""
			t.runTarget = ""
			t.streaming = false
		}
		t.popup = popup
		return t, nil

	case ExecProcessDone:
		return t, nil

	case SessionSelect:
		next, cmd := t.runCommandSwitch(msg.id)
		if next.onceCall {
			return next, chainSingleShotSubmit(cmd, next.userInput)
		}
		return next, cmd

	case SessionNew:
		next, cmd, _ := t.commandNew(nil)
		return next, cmd

	case SessionNewSubmit:
		next, cmd := t.showNewPromptPicker(msg.name)
		return next, cmd

	case SessionNewCustomSubmit:
		next, cmd := t.showNewCustomPopup(msg.name)
		return next, cmd

	case SessionNewPromptSubmit:
		next, cmd := t.runCreateSession(msg.name, msg.body)
		if next.onceCall {
			return next, chainSingleShotSubmit(cmd, next.userInput)
		}
		return next, cmd

	case ModelScopeSelect:
		switch msg.scope {
		case "add":
			next, cmd, _ := t.commandModelAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandModelRemove()
			return next, cmd
		case "session":
			next, cmd, _ := t.commandSessionModel()
			return next, cmd
		case "dispatch":
			next, cmd, _ := t.commandDispatcher()
			return next, cmd
		case "summary":
			next, cmd, _ := t.commandSummaryModel()
			return next, cmd
		case "reasoning":
			next, cmd, _ := t.commandReasoning(nil)
			return next, cmd
		}
		return t, nil

	case McpAction:
		switch msg.action {
		case "add":
			next, cmd, _ := t.commandMcpAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandMcpRemove()
			return next, cmd
		case "install":
			next, cmd, _ := t.commandMcpInstall()
			return next, cmd
		}
		return t, nil

	case McpInstallPick:
		if msg.index < 0 || msg.index >= len(mcpInstallClients) {
			return t, tea.Println(errorStyle.Render("[!] invalid selection") + "\n")
		}
		c := mcpInstallClients[msg.index]
		return t, func() tea.Msg { return runMcpInstall(c) }

	case McpInstallDone:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] %s: %v", msg.client, msg.err)) + "\n")
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("✓ added agenvoy to %s (%s)", msg.client, msg.path)) + "\n")

	case McpRemove:
		return t.runMcpRemove(msg)

	case McpAddName:
		if msg.name == "" {
			t.mcpAdd = nil
			return t, tea.Println(errorStyle.Render("[!] mcp name required") + "\n")
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
		case "http":
			next, cmd := t.openMcpAddURL()
			return next, cmd
		}
		t.mcpAdd = nil
		return t, nil

	case McpAddCommand:
		if msg.command == "" {
			t.mcpAdd = nil
			return t, tea.Println(errorStyle.Render("[!] command required") + "\n")
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
		next, cmd := t.openMcpAddScope()
		return next, cmd

	case McpAddURL:
		if msg.url == "" {
			t.mcpAdd = nil
			return t, tea.Println(errorStyle.Render("[!] url required") + "\n")
		}
		t.mcpAdd.url = msg.url
		next, cmd := t.openMcpAddAuthMethod()
		return next, cmd

	case McpAddAuthMethod:
		switch msg.method {
		case "none":
			next, cmd := t.openMcpAddHeaders()
			return next, cmd
		case "bearer":
			next, cmd := t.openMcpAddBearerToken()
			return next, cmd
		case "apikey":
			next, cmd := t.openMcpAddAPIKeyHeader()
			return next, cmd
		case "basic":
			next, cmd := t.openMcpAddBasicToken()
			return next, cmd
		}
		t.mcpAdd = nil
		return t, nil

	case McpAddBearerToken:
		t.mcpAdd.headers = bearerAuthorizationHeader(msg.token)
		next, cmd := t.openMcpAddHeaders()
		return next, cmd

	case McpAddAPIKeyHeader:
		t.mcpAdd.authHeaderName = msg.header
		if t.mcpAdd.authHeaderName == "" {
			t.mcpAdd.authHeaderName = "X-API-Key"
		}
		next, cmd := t.openMcpAddAPIKeyValue(t.mcpAdd.authHeaderName)
		return next, cmd

	case McpAddAPIKeyValue:
		t.mcpAdd.headers = apiKeyHeader(t.mcpAdd.authHeaderName, msg.value)
		next, cmd := t.openMcpAddHeaders()
		return next, cmd

	case McpAddBasicToken:
		t.mcpAdd.headers = basicAuthorizationHeader(msg.token)
		next, cmd := t.openMcpAddHeaders()
		return next, cmd

	case McpAddHeaders:
		t.mcpAdd.headers = mergeMcpHeaders(t.mcpAdd.headers, parseKV(msg.raw))
		next, cmd := t.openMcpAddScope()
		return next, cmd

	case McpAddScope:
		t.mcpAdd.scope = msg.scope
		switch msg.scope {
		case "global":
			return t.finalizeMcpAdd()
		case "session":
			next, cmd := t.openMcpAddSessionPick()
			return next, cmd
		}
		t.mcpAdd = nil
		return t, nil

	case McpAddSessionPick:
		t.mcpAdd.sessionID = msg.id
		return t.finalizeMcpAdd()

	case McpAddSaved:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] mcp add: %v", msg.err)) + "\n")
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ mcp added: %s (%s) · restart daemon to apply", msg.name, msg.scope)) + "\n")

	case ReasoningScopeSelect:
		switch msg.scope {
		case "global":
			next, cmd := t.openReasoningGlobalPopup()
			return next, cmd
		case "session":
			next, cmd := t.openReasoningSessionPopup()
			return next, cmd
		}
		return t, nil

	case AllowSkillScopeSelect:
		next, cmd := t.openAllowSkillPickerPopup(msg.scope)
		return next, cmd

	case AllowSkillPick:
		next, cmd := t.runAllowSkillToggle(msg.scope, msg.name)
		return next, cmd

	case AllowCmdSubmit:
		next, cmd := t.runAllowCmdAppend(msg.name)
		return next, cmd

	case ModelRemove:
		next, cmd := t.runModelRemove(msg.chosen)
		return next, cmd

	case BotNameSubmit:
		sid := strings.TrimSpace(t.currentSessionID)
		if sid == "" {
			t.botBodyDraft = ""
			return t, tea.Println(errorStyle.Render("[!] no current session") + "\n")
		}
		if cmd, ok := t.botCheckConflict(sid, msg.name); !ok {
			t.botBodyDraft = ""
			return t, cmd
		}
		next, cmd := t.showBotPromptPicker(msg.name)
		return next, cmd

	case BotCustomSubmit:
		next, cmd := t.showBotCustomPopup(msg.name)
		return next, cmd

	case BotPromptSubmit:
		sid := strings.TrimSpace(t.currentSessionID)
		if sid == "" {
			return t, tea.Println(errorStyle.Render("[!] no current session") + "\n")
		}
		return t, t.botSaveCmd(sid, msg.name, msg.body)

	case BotSaved:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] bot save: %v", msg.err)) + "\n")
		}
		if t.currentSessionID != "" {
			t.currentSessionName = msg.name
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ bot saved: %s", msg.name)) + "\n")

	case ModelAddProviderPick:
		return t.runModelAddProviderPick(msg.provider)

	case GrokMethodPick:
		if t.modelAdd == nil {
			return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
		}
		switch msg.method {
		case "grok-oauth":
			t.modelAdd.provider = "grok-oauth"
			return t.modelAddViaOAuth()
		default:
			t.modelAdd.provider = "grok"
			return t.openModelAddAPIKey()
		}

	case OpenAIMethodPick:
		if t.modelAdd == nil {
			return t, tea.Println(errorStyle.Render("[!] model add state lost") + "\n")
		}
		switch msg.method {
		case "codex":
			t.modelAdd.provider = "codex"
			return t.modelAddViaOAuth()
		default:
			t.modelAdd.provider = "openai"
			return t.openModelAddAPIKey()
		}

	case ModelAddAPIKeyReplace:
		return t.runModelAddAPIKeyReplace(msg.replace)

	case ModelAddAPIKeySubmit:
		return t.runModelAddAPIKeySubmit(msg.key)

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

	case ModelScanLocalResult:
		return t.runModelScanLocalResult(msg)

	case ModelScanLocalPick:
		return t.runModelScanLocalPick(msg.chosen)

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
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] add-model: %v", msg.err))+"\n"))
		} else {
			agents.Reload()
			seq = append(seq, tea.Println(hintStyle.Render("⎯ model added · registry reloaded")+"\n"))
		}
		return t, tea.Sequence(seq...)

	case DiscordAction:
		switch msg.action {
		case "enable":
			next, cmd := t.openDiscordTokenPrompt()
			return next, cmd
		case "disable":
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ discord disabling")+"\n"),
				disableDiscord(),
			)
		}
		return t, nil

	case DiscordTokenSubmit:
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ discord verifying token (≤10s)")+"\n"),
			enableDiscord(msg.token),
		)

	case TelegramAction:
		switch msg.action {
		case "enable":
			next, cmd := t.openTelegramTokenPrompt()
			return next, cmd
		case "disable":
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ telegram disabling")+"\n"),
				disableTelegram(),
			)
		}
		return t, nil

	case TelegramTokenSubmit:
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ telegram verifying token (≤10s)")+"\n"),
			enableTelegram(msg.token),
		)

	case CronAction:
		switch msg.action {
		case "add":
			next, cmd, _ := t.commandCronAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandCronRemove()
			return next, cmd
		case "edit":
			next, cmd, _ := t.commandCronEdit()
			return next, cmd
		}
		return t, nil

	case CronAddSubmit:
		next, cmd := t.runCronAddSubmit(msg.requirement)
		return next, cmd

	case CronRemoveSelect:
		if len(msg.skills) == 0 {
			return t, tea.Println(hintStyle.Render("⎯ no crons selected") + "\n")
		}
		next, cmd := t.runCronRemove(msg.skills)
		return next, cmd

	case CronEditSelect:
		next, cmd := t.openCronEditRequirement(msg.skill, msg.expression)
		return next, cmd

	case CronEditSubmit:
		next, cmd := t.runCronEditSubmit(msg.skill, msg.expression, msg.requirement)
		return next, cmd

	case TaskAction:
		switch msg.action {
		case "add":
			next, cmd, _ := t.commandTaskAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandTaskRemove()
			return next, cmd
		case "edit":
			next, cmd, _ := t.commandTaskEdit()
			return next, cmd
		}
		return t, nil

	case TaskAddSubmit:
		next, cmd := t.runTaskAddSubmit(msg.requirement)
		return next, cmd

	case TaskRemoveSelect:
		tasks := listTaskEntries()
		if msg.idx < 0 || msg.idx >= len(tasks) {
			return t, tea.Println(errorStyle.Render("[!] task index out of range") + "\n")
		}
		next, cmd := t.openTaskRemoveConfirm(tasks[msg.idx].Skill)
		return next, cmd

	case TaskRemoveConfirm:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ task remove cancelled") + "\n")
		}
		next, cmd := t.runTaskRemove(msg.skill)
		return next, cmd

	case RemoveSessionPick:
		return t.runRemoveSessionPick(msg.chosen)

	case RemoveSessionConfirm:
		return t.runRemoveSessionConfirm(msg)

	case ResetSessionConfirm1:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ reset cancelled") + "\n")
		}
		next, cmd := t.openResetConfirm2(msg.id)
		return next, cmd

	case ResetSessionConfirm2:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ reset cancelled") + "\n")
		}
		next, cmd := t.runResetSession(msg.id)
		return next, cmd

	case ResetSessionDone:
		next, cmd := t.finishResetSession(msg)
		return next, cmd

	case SummaryDone:
		next, cmd := t.finishSummary(msg)
		return next, cmd

	case TaskEditSelect:
		next, cmd := t.openTaskEditRequirement(msg.skill, msg.at)
		return next, cmd

	case TaskEditSubmit:
		next, cmd := t.runTaskEditSubmit(msg.skill, msg.at, msg.requirement)
		return next, cmd

	case DiscordDone:
		t.discordStatus = getDiscordStatus()
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] discord %s: %v", msg.action, msg.err))+"\n"))
		} else {
			seq = append(seq, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ discord %sd · daemon reloading", msg.action))+"\n"))
		}
		return t, tea.Sequence(seq...)

	case TelegramDone:
		t.telegramStatus = getTelegramStatus()
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] telegram %s: %v", msg.action, msg.err))+"\n"))
		} else {
			seq = append(seq, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ telegram %sd · daemon reloading", msg.action))+"\n"))
		}
		return t, tea.Sequence(seq...)

	case DangerousSelect:
		switch msg.action {
		case "remove-session":
			next, cmd, _ := t.commandRemoveSession()
			return next, cmd
		case "allow-skill":
			next, cmd, _ := t.commandAllowSkill(nil)
			return next, cmd
		case "allow-cmd":
			next, cmd, _ := t.commandAllowCmd(nil)
			return next, cmd
		case "allow-report":
			next, cmd, _ := t.commandAllowReport(nil)
			return next, cmd
		}
		return t, nil

	case FeatureSelect:
		switch msg.feature {
		case "voice":
			next, cmd, _ := t.commandVoice(nil)
			return next, cmd
		case "image2":
			next, cmd, _ := t.commandImage2(nil)
			return next, cmd
		case "kuradb":
			next, cmd, _ := t.commandKuradb(nil)
			return next, cmd
		}
		return t, nil

	case VoiceAction:
		if msg.action == "enable" && voiceNeedsGeminiKey() {
			next, cmd := t.openVoiceKeyPrompt()
			return next, cmd
		}
		return t, setVoice(msg.action)

	case VoiceKeySubmit:
		token := strings.TrimSpace(msg.token)
		if token == "" {
			return t, tea.Println(errorStyle.Render("[!] voice enable: GEMINI_API_KEY is required") + "\n")
		}
		if err := keychain.Set("GEMINI_API_KEY", token); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] voice keychain.Set: %v", err)) + "\n")
		}
		if err := config.SaveKey("GEMINI_API_KEY"); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] voice session.SaveKey: %v", err)) + "\n")
		}
		return t, setVoice("enable")

	case VoiceDone:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] voice %s: %v", msg.action, msg.err)) + "\n")
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ voice %sd", msg.action)) + "\n")

	case Image2Action:
		if msg.action == "enable" && !openaicodex.HasToken() {
			next, cmd := t.startImage2CodexOAuth()
			return next, cmd
		}
		return t, setImage2(msg.action)

	case Image2Done:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] image2 %s: %v", msg.action, msg.err)) + "\n")
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ image2 %sd", msg.action)) + "\n")

	case KuradbAction:
		switch msg.action {
		case "enable":
			if strings.TrimSpace(keychain.Get("OPENAI_API_KEY")) == "" {
				next, cmd := t.openKuradbKeyPrompt()
				return next, cmd
			}
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ kuradb installing")+"\n"),
				runKuradbEnableExec(),
			)
		case "disable":
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ kuradb removing")+"\n"),
				runKuradbDisableExec(),
			)
		case "update":
			return t, tea.Sequence(
				tea.Println(hintStyle.Render("⎯ kuradb updating")+"\n"),
				runKuradbUpdateExec(),
			)
		}
		return t, nil

	case KuradbKeySubmit:
		token := strings.TrimSpace(msg.token)
		if token == "" {
			return t, tea.Println(errorStyle.Render("[!] kuradb enable: OPENAI_API_KEY is required") + "\n")
		}
		if err := keychain.Set("OPENAI_API_KEY", token); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb keychain.Set: %v", err)) + "\n")
		}
		if err := kuradb.SyncOpenAIKey(token); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb SyncOpenAIKey: %v", err)) + "\n")
		}
		if err := config.SaveKey("OPENAI_API_KEY"); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb session.SaveKey: %v", err)) + "\n")
		}
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ kuradb installing")+"\n"),
			runKuradbEnableExec(),
		)

	case AdminChannelSubmit:
		value := strings.TrimSpace(msg.value)
		if value != "" {
			if _, _, ok := exec.ParseAdminChannel(value); !ok {
				return t, tea.Println(errorStyle.Render("[!] admin-channel: format must be tg@<chatID> or dc@<channelID>") + "\n")
			}
		}
		cfg, err := config.Load()
		if err != nil || cfg == nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] admin-channel: session.Load: %v", err)) + "\n")
		}
		cfg.AdminChannel = value
		if err := config.Save(cfg); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] admin-channel: session.Save: %v", err)) + "\n")
		}
		if value == "" {
			return t, tea.Println(hintStyle.Render("⎯ admin-channel cleared") + "\n")
		}
		return t, tea.Println(hintStyle.Render("⎯ admin-channel set · "+value) + "\n")

	case AllowReportAction:
		next, cmd := t.openAllowReportConfirm(msg.action)
		return next, cmd

	case AllowReportConfirm:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ allow-report cancelled") + "\n")
		}
		if msg.action == "enable" {
			next, cmd := t.runAllowReportEnable()
			return next, cmd
		}
		next, cmd := t.runAllowReportDisable()
		return next, cmd

	case KeySelect:
		next, cmd := t.openKeyValuePrompt(msg.key)
		return next, cmd

	case KeySubmit:
		if msg.value == "" {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] key %s: value is required", msg.key)) + "\n")
		}
		if err := keychain.Set(msg.key, msg.value); err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] keychain.Set %s: %v", msg.key, err)) + "\n")
		}
		if msg.key == "OPENAI_API_KEY" {
			if err := kuradb.SyncOpenAIKey(msg.value); err != nil {
				return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb SyncOpenAIKey: %v", err)) + "\n")
			}
		}
		return t, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ %s updated", msg.key)) + "\n")

	case KuradbDone:
		if msg.err != nil {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] kuradb %s: %v", msg.action, msg.err)) + "\n")
		}
		hint := fmt.Sprintf("⎯ kuradb %sd · daemon reloading", msg.action)
		if msg.action == "enable" {
			hint += " · restart agen to load RAG tools"
		}
		return t, tea.Println(hintStyle.Render(hint) + "\n")

	case DispatcherSelect:
		next, cmd := t.runDispatcherSelect(msg.name)
		agents.Reload()
		return next, cmd

	case SummaryModelSelect:
		next, cmd := t.runSummaryModelSelect(msg.name)
		agents.Reload()
		return next, cmd

	case ReasoningSelect:
		next, cmd := t.runReasoningSelect(msg.level)
		return next, cmd

	case SessionModelSelect:
		next, cmd := t.runSessionModelSelect(msg.model)
		return next, cmd

	case SessionReasoningSelect:
		next, cmd := t.runSessionReasoningSelect(msg.reasoning)
		return next, cmd

	case UpdateConfirm:
		if !msg.ok {
			return t, tea.Println(hintStyle.Render("⎯ update cancelled") + "\n")
		}
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ stopping daemon · downloading latest · expect sudo prompt")+"\n"),
			runUpdateExec(),
		)

	case UpdateDone:
		t.quitting = true
		if msg.err != nil {
			return t, tea.Sequence(
				tea.Println(errorStyle.Render(fmt.Sprintf("[!] update: %v", msg.err))+"\n"),
				tea.Quit,
			)
		}
		return t, tea.Quit

	case LogDone:
		if msg.err != nil {
			return t, tea.Sequence(
				tea.ClearScreen,
				tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
				tea.Println(errorStyle.Render(fmt.Sprintf("[!] log: %v", msg.err))+"\n"),
			)
		}
		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		)

	case StartupSelectSession:
		popup := popupSwitch("")
		if popup == nil {
			return t, nil
		}
		popup.title = "Pick session to attach"
		popup.onConfirm = func(chosen string) any {
			if chosen == "" {
				return SessionNew{}
			}
			return StartupSessionSelect{id: chosen}
		}
		t.popup = popup
		return t, nil

	case StartupSessionSelect:
		t.currentSessionID = msg.id
		t.currentSessionName, _ = configBot.Get(msg.id)
		t.inputHistory = loadInputHistory(msg.id)
		t.inputHistoryIdx = -1
		t = t.restartTailer()
		t.tokens = 0
		t.lastIn = 0
		t.lastOut = 0
		t.currentModel = ""
		t.activity = ""

		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		}
		seq = append(seq, loadSessionTail(msg.id)...)
		return t, tea.Sequence(seq...)

	case tailLine:
		if t.onceCall {
			return t, nil
		}
		return t, tea.Println(msg.line)

	case Log:
		if t.onceCall {
			return t, nil
		}
		return t, tea.Println(renderLogLine(msg))

	case initTailer:
		return t.restartTailer(), nil

	case released:
		if t.onceCall || msg.tag == "" || msg.tag == projectVersion || projectVersion == "dev" {
			return t, nil
		}

		hint := okayStyle.Render("⏺ latest: "+msg.tag) + hintStyle.Render("  (now is ") + textStyle.Render(projectVersion) + hintStyle.Render(")")
		return t, tea.Println(hint + "\n")

	case spinner.TickMsg:
		if t.running {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			cmds = append(cmds, cmd)
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

func (t TUI) startResume(msg ResumeExec) (tea.Model, tea.Cmd) {
	sid := msg.SessionID
	if sid == "" {
		sid = t.currentSessionID
	}
	t.running = true
	t.runStartedAt = time.Now()
	t.runTarget = ""
	go runExec(t.ctx, msg.Content, t.allowAll, t.cwd, sid, msg.PendingTask)
	return t, t.spinner.Tick
}

func noMatches(input string) bool {
	if scanner := agents.Scanner(); scanner != nil {
		if m, _ := runtime.MatchSkill(scanner, input); m != nil {
			return true
		}
	}
	if agent, _, _ := external.MatchExternal(input); agent != "" {
		return true
	}
	return false
}

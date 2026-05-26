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
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
)

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case tea.KeyTab:
			if t.selector != nil {
				t = t.selectCommand()
				return t, nil
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
			}

			t.running = true
			t.runStartedAt = time.Now()
			t.runTarget = targetSession(content, t.currentSessionID)

			go runExec(t.ctx, content, false, t.cwd, t.currentSessionID, t.mode == webMode)

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
			t.currentSessionName, _ = session.GetBot(t.currentSessionID)
		}
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			return t, tea.Println(errorStyle.Render(fmt.Sprintf("[!] exec error: %v", msg.err)) + "\n")
		}
		return t, nil

	case WorkDir:
		t.cwd = msg.dir
		return t, nil

	case agentEvent:
		return t.handleAgentEvent(msg.event)

	case Pending:
		popup := newPopup(msg.id, msg.request)
		if popup == nil {
			runtime.Resolve(msg.id, runtime.Reply{Error: fmt.Errorf("invalid pending request")})
			return t, nil
		}

		t.popup = popup
		return t, nil

	case ModeSelect:
		return t.runModeSelect(msg.mode)

	case SessionSelect:
		next, cmd := t.runCommandSwitch(msg.id)
		return next, cmd

	case SessionNew:
		next, cmd, _ := t.commandNew(nil)
		return next, cmd

	case SessionNewSubmit:
		return t.runCreateSession(msg.name)

	case ModelScopeSelect:
		switch msg.scope {
		case "global":
			next, cmd := t.openModelGlobalPopup()
			return next, cmd
		case "session":
			next, cmd, _ := t.commandSessionModel()
			return next, cmd
		}
		return t, nil

	case ModelAction:
		switch msg.action {
		case "add":
			next, cmd, _ := t.commandModelAdd()
			return next, cmd
		case "remove":
			next, cmd, _ := t.commandModelRemove()
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
		}
		return t, nil

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
		next, cmd := t.openMcpAddHeaders()
		return next, cmd

	case McpAddHeaders:
		t.mcpAdd.headers = parseKV(msg.raw)
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

	case ModelRemove:
		next, cmd := t.runModelRemove(msg.name)
		agents.Reload()
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
		next, cmd := t.openBotBodyPopup(msg.name)
		return next, cmd

	case BotBodySubmit:
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

	case ModelAddDone:
		seq := []tea.Cmd{
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
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
		next, cmd := t.openCronRemoveConfirm(msg.skill)
		return next, cmd

	case CronRemoveConfirm:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ cron remove cancelled") + "\n")
		}
		next, cmd := t.runCronRemove(msg.skill)
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

	case RemoveSessionConfirm1:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ remove-session cancelled") + "\n")
		}
		next, cmd := t.openRemoveSessionConfirm2(msg.id)
		return next, cmd

	case RemoveSessionConfirm2:
		if !msg.yes {
			return t, tea.Println(hintStyle.Render("⎯ remove-session cancelled") + "\n")
		}
		next, cmd := t.runRemoveSession(msg.id)
		return next, cmd

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
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
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
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
		}
		if msg.err != nil {
			seq = append(seq, tea.Println(errorStyle.Render(fmt.Sprintf("[!] telegram %s: %v", msg.action, msg.err))+"\n"))
		} else {
			seq = append(seq, tea.Println(hintStyle.Render(fmt.Sprintf("⎯ telegram %sd · daemon reloading", msg.action))+"\n"))
		}
		return t, tea.Sequence(seq...)

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
		return t, tea.Sequence(
			tea.Println(hintStyle.Render("⎯ kuradb installing")+"\n"),
			runKuradbEnableExec(),
		)

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
				tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
				tea.Println(errorStyle.Render(fmt.Sprintf("[!] log: %v", msg.err))+"\n"),
			)
		}
		return t, tea.Sequence(
			tea.ClearScreen,
			tea.Println(headerBlock(t.cwd, t.daemonStatus, t.httpStatus, t.discordStatus, t.telegramStatus)),
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
			return SessionSelect{id: chosen}
		}
		t.popup = popup
		return t, nil

	case LoadHistoryCheck:
		sid := msg.id
		t.popup = &Popup{
			kind:    popupSingleSelect,
			title:   "Load previous session history?",
			options: []string{"Yes", "No"},
			values:  []string{"yes", "no"},
			cursor:  1,
			onConfirm: func(chosen string) any {
				return LoadHistorySelect{id: sid, load: chosen == "yes"}
			},
		}
		return t, nil

	case LoadHistorySelect:
		if !msg.load {
			return t, nil
		}
		return t, tea.Sequence(loadSessionTail(msg.id)...)

	case tailLine:
		if t.mode != cliMode {
			return t, nil
		}
		return t, tea.Println(msg.line)

	case Log:
		if t.mode != cliMode {
			return t, nil
		}
		return t, tea.Println(renderLogLine(msg))

	case initTailer:
		return t.restartTailer(), nil

	case released:
		if msg.tag == "" || msg.tag == projectVersion || projectVersion == "dev" {
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

package exec

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	go_pkg_keychain "github.com/pardnchiu/go-pkg/filesystem/keychain"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	agentsexecallowskill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/compact"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/fast"
	execmodel "github.com/pardnchiu/agenvoy/internal/agents/exec/model"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/retryHandler"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	sessionmanager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	sessionhistory "github.com/pardnchiu/agenvoy/internal/session/history"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
	"github.com/pardnchiu/agenvoy/internal/tools"
	audiotool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
	imagetool "github.com/pardnchiu/agenvoy/internal/tools/external/image"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type ExecuteMeta struct {
	Agent             agenttypes.Agent
	FallbackAgents    []agenttypes.Agent
	Model             string
	WorkDir           string
	Skill             *skill.Skill
	SkillName         string
	SkillScanner      *runtime.SkillScanner
	Task              string
	Input             string
	SessionID         string
	ImageInputs       []string
	FileInputs        []string
	ExcludeTools      []string
	ExcludeSkills     []string
	ClientTools       []go_llm_router.Tool
	ExtraSystemPrompt string
	Reasoning         string
	AllowAll          bool
	TUI               bool
	PendingTask       string
	TaskHash          string
	KeepPending       bool
	IgnoreHistory     bool
	ReplyMessageID    string
	HistoryContent    string
	Sender            string
}

const (
	sendStopGrace   = 3 * time.Second
	fanoutSendGrace = 3 * time.Second
	fanoutStopGrace = 3 * time.Second
)

type (
	AllowAllCtxKey   struct{}
	ParentEventsKey  struct{}
	ParentWorkDirKey struct{}
)

func Execute(ctx context.Context, data ExecuteMeta, session *agenttypes.Session, events chan<- agenttypes.Event, allowAll bool) (execErr error) {
	execCtx := go_llm_router.WithSessionID(agenttypes.WithSessionID(ctx, session.ID), session.ID)
	execStart := time.Now()

	if !allowAll {
		if data.Skill != nil && strings.TrimSpace(data.Skill.Content) != "" && agentsexecallowskill.Match(data.WorkDir, data.Skill.Name) {
			allowAll = true
		}
	}

	execCtx = context.WithValue(execCtx, AllowAllCtxKey{}, allowAll)

	if events != nil {
		execCtx = context.WithValue(execCtx, ParentEventsKey{}, events)
	}

	if strings.TrimSpace(data.WorkDir) != "" {
		execCtx = context.WithValue(execCtx, ParentWorkDirKey{}, data.WorkDir)
	}

	pushCtx := execCtx
	execCtx, execCancel := context.WithCancelCause(execCtx)
	defer execCancel(nil)

	var runTaskHash *atomic.Pointer[string]
	windowHash := agenttypes.WindowHash(ctx)
	reasoningRef := &atomic.Pointer[string]{}
	if session.ID != "" {
		if err := sessionmanager.AddConcurrent(execCtx, session.ID); err != nil {
			return utilsmsg.Wrap(utilsmsg.Canceled, "EnterConcurrent", "", err)
		}
		defer sessionmanager.RemoveConcurrent(session.ID)
		defer markRunning(session.ID)()
		defer ClearSteer(session.ID)

		stateless := session.Stateless
		terminalRecorded := &atomic.Bool{}
		defer func() {
			if execErr == nil || stateless || terminalRecorded.Load() {
				return
			}
			errEvent := agenttypes.ErrorEvent(execErr)
			errEvent.WindowHash = windowHash
			sessionlog.Record(session.ID, errEvent)
		}()

		original := events
		fanoutEvents := make(chan agenttypes.Event, 64)
		done := make(chan struct{})
		sid := session.ID
		pushHook, hasPush := lookupPushHook(sid)
		isDcPush := hasPush && !isDcPushSuppressed(pushCtx)
		scheduleName := ""
		if IsSchedule(pushCtx) {
			scheduleName = ScheduleLabel(DCPushPrefix(pushCtx))
		}
		var pushTextBuf strings.Builder
		var pushDoneEv agenttypes.Event
		runTaskHash = &atomic.Pointer[string]{}
		taskHashRef := runTaskHash
		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event fanout goroutine panic recovered",
						slog.String("session", sid),
						slog.Any("panic", r))
				}
			}()
			for ev := range fanoutEvents {
				if ev.WindowHash == "" {
					ev.WindowHash = windowHash
				}
				if ev.TaskHash == "" && ev.Source == "" {
					if h := taskHashRef.Load(); h != nil {
						ev.TaskHash = *h
					}
				}
				if ev.Type == agenttypes.EventDone && ev.Source == "" {
					if r := reasoningRef.Load(); r != nil {
						ev.Reasoning = *r
					}
				}
				if scheduleName != "" && ev.Source == "" && ev.Model != "" {
					ev.Model = scheduleName
				}
				if ev.Source == "" {
					switch ev.Type {
					case agenttypes.EventDone, agenttypes.EventCanceled, agenttypes.EventError, agenttypes.EventExecError:
						terminalRecorded.Store(true)
					}
				}
				if !stateless && ev.Source == "" {
					sessionlog.Record(sid, ev)
				}
				if isDcPush {
					switch ev.Type {
					case agenttypes.EventText:
						if ev.Source == "" && ev.Text != "" {
							if pushTextBuf.Len() > 0 {
								pushTextBuf.WriteByte('\n')
							}
							pushTextBuf.WriteString(ev.Text)
						}
					case agenttypes.EventDone:
						if ev.Source == "" {
							pushDoneEv = ev
						}
					}
				}
				select {
				case original <- ev:
				case <-execCtx.Done():
					select {
					case original <- ev:
					case <-time.After(fanoutSendGrace):
						return
					}
				}
			}
		}()
		defer func() {
			close(fanoutEvents)
			select {
			case <-done:
			case <-time.After(fanoutStopGrace):
				slog.Warn("event fanout did not drain; abandoning it",
					slog.String("session", sid))
			}
			if isDcPush {
				text := strings.TrimSpace(pushTextBuf.String())
				if text != "" {
					pushHook(pushCtx, PushPayload{
						SessionID:     sid,
						Text:          text,
						Model:         pushDoneEv.Model,
						Reasoning:     pushDoneEv.Reasoning,
						Usage:         pushDoneEv.Usage,
						Duration:      pushDoneEv.Duration,
						OutputElapsed: pushDoneEv.OutputElapsed,
						Prefix:        DCPushPrefix(pushCtx),
					})
				}
			}
		}()
		events = fanoutEvents
	}

	// * if skill is empty, then treat as no skill
	if data.Skill != nil && data.Skill.Content == "" {
		data.Skill = nil
	}

	scanner := data.SkillScanner
	if scanner == nil {
		scanner = agents.Scanner()
	}

	exec, err := tools.NewExecutor(data.WorkDir, session.ID, scanner)
	if err != nil {
		return utilsmsg.Trace("tools.NewExecutor", "", err)
	}

	emitChangedFiles := func() {
		if files := exec.EditedFiles(); len(files) > 0 {
			events <- agenttypes.Event{Type: agenttypes.EventFileChanged, Files: files}
		}
	}

	exec.CancelExecution = func() { execCancel(nil) }
	exec.IgnoreHistory = data.IgnoreHistory

	keepPending := true
	if !session.Stateless && session.ID != "" {
		if data.PendingTask != "" {
			exec.PendingTask = data.PendingTask
			exec.SeedFiles(interactive.LoadPendingFiles(session.ID, data.PendingTask))
		} else {
			objective := data.Task
			if objective == "" {
				if s, ok := session.UserMessage.Content.(string); ok {
					objective = s
				}
			}
			exec.PendingTask = interactive.CreateExecPending(session.ID, data.TaskHash, objective, data.ReplyMessageID, data.Agent.Name(), data.Reasoning, allowAll)
		}
		defer func() {
			if keepPending || data.KeepPending {
				if err := interactive.WriteTodos(session.ID, exec.PendingTask, nil); err != nil {
					slog.Debug("interactive.WriteTodos",
						slog.String("session", session.ID),
						slog.String("error", err.Error()))
				}
				return
			}
			if running, unreported := interactive.BackgroundState(session.ID, exec.PendingTask); running+unreported > 0 {
				publishPending(session.ID, deliverFor(ctx, session.ID), exec.PendingTask)
				return
			}
			interactive.CleanupPending(session.ID, exec.PendingTask)
		}()
		defer historystore.ClearTaskOrigin(exec.PendingTask)
		defer interactive.KeepOnline(session.ID, exec.PendingTask)()
		registerCancel(exec.PendingTask, execCancel)
		defer unregisterCancel(exec.PendingTask)
		if runTaskHash != nil {
			hash := exec.PendingTask
			runTaskHash.Store(&hash)
		}
	}

	cfg, _ := config.Load()
	if !imagetool.Enabled() {
		data.ExcludeTools = append(data.ExcludeTools, "generate_image")
	}
	if !audiotool.TTSEnabled() {
		data.ExcludeTools = append(data.ExcludeTools, "generate_audio")
	}
	if (cfg == nil || !cfg.TelegramEnabled || go_pkg_keychain.Get("TELEGRAM_TOKEN") == "") &&
		(cfg == nil || !cfg.DiscordEnabled || go_pkg_keychain.Get("DISCORD_TOKEN") == "") {
		data.ExcludeTools = append(data.ExcludeTools,
			"list_chatbot", "send_to_chatbot")
	}

	if claudeCode.Is(data.Agent.Name()) {
		exec.Tools = slices.DeleteFunc(slices.Clone(exec.AllTools), func(t go_llm_router.Tool) bool {
			return t.Function.Name == "run_tool"
		})
	}

	if len(data.ExcludeTools) > 0 {
		excluded := make(map[string]bool, len(data.ExcludeTools))
		var prefixes []string
		for _, name := range data.ExcludeTools {
			if prefix, ok := strings.CutSuffix(name, "*"); ok {
				prefixes = append(prefixes, prefix)
				continue
			}
			excluded[name] = true
		}
		if len(prefixes) > 0 {
			for _, t := range exec.AllTools {
				if slices.ContainsFunc(prefixes, func(p string) bool {
					return strings.HasPrefix(t.Function.Name, p)
				}) {
					excluded[t.Function.Name] = true
				}
			}
		}
		exec.ExcludeTools = excluded

		filtered := exec.Tools[:0]
		for _, t := range exec.Tools {
			if !excluded[t.Function.Name] {
				filtered = append(filtered, t)
			}
		}
		exec.Tools = filtered

		for name := range excluded {
			delete(exec.UnmarkedTools, name)
		}
	}

	var toolNames []string
	if slices.ContainsFunc(exec.Tools, func(t go_llm_router.Tool) bool { return t.Function.Name == "run_tool" }) {
		for _, t := range exec.AllTools {
			if name := t.Function.Name; !slices.Contains(configs.SYSTEM_DEFAULT_TOOLS, name) && !exec.ExcludeTools[name] {
				toolNames = append(toolNames, name)
			}
		}
	}
	assignTurnContext(ctx, session, data.WorkDir, allowAll, scanner, data.ExcludeSkills, !exec.ExcludeTools["run_skill"], toolNames)
	if data.Skill != nil {
		assignSkill(session, data.Skill)
	}

	clientTools := make(map[string]bool, len(data.ClientTools))
	for _, t := range data.ClientTools {
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		clientTools[name] = true
		exec.Tools = append(exec.Tools, t)
	}

	if claudeCode.Is(data.Agent.Name()) {
		for _, t := range exec.Tools {
			if name := t.Function.Name; name != "find_tools" && !clientTools[name] {
				exec.UnmarkedTools[name] = true
			}
		}
	}

	limit := configs.MAX_TOOL_ITERATIONS
	reasoning := ResolveReasoning(session.ID, data.Reasoning)
	reasoningLabel := reasoning.String()
	reasoningRef.Store(&reasoningLabel)

	pool := make([]agenttypes.Agent, 0, 1+len(data.FallbackAgents))
	pool = append(pool, data.Agent)
	pool = append(pool, data.FallbackAgents...)
	retryState := execmodel.RetryState{Agents: pool}

	var usage go_llm_router.Usage
	var sendElapsedTotal time.Duration
	alreadyCall := make(map[string]string)
	turnAllowAll := false
	emptyCount := 0
	compactFailed := false
	lastInputTokens := 0
	var shownReasoning []string
	type sendOutcome struct {
		resp        *go_llm_router.Output
		code        int
		err         error
		textEmitted bool
		reasoned    bool
	}
	sendFailCount := 0
	timeoutRetryCount := 0
	rateLimitRetryCount := 0
	oldHistoriesCompacted := false
	firstAttempt := true

	compact.Warm(execCtx)

	compactHistory := func() {
		compacted := false
		if !oldHistoriesCompacted {
			compacted = compact.ExtractContextHistories(execCtx, data.Agent, session, &usage, events)
			oldHistoriesCompacted = true
		}
		if !compacted {
			events <- agenttypes.Event{Type: agenttypes.EventCompact, Text: "tool_call"}
			compacted = compact.ToolHistory(execCtx, data.Agent, session, &usage, exec.PendingTask)
		}
		if compacted {
			lastInputTokens = 0
		} else {
			compactFailed = true
		}
	}

	for range limit {
		if execCtx.Err() != nil {
			events <- agenttypes.Event{Type: agenttypes.EventCanceled, Model: data.Agent.Name(), Duration: time.Since(execStart)}
			if errors.Is(context.Cause(execCtx), runtime.ErrUserCanceled) {
				interactive.DeletePending(session.ID, exec.PendingTask)
			}
			return execCtx.Err()
		}
		if pending := getSteer(session.ID); len(pending) > 0 {
			raw := strings.Join(pending, "\n")
			session.ToolHistories = append(session.ToolHistories, go_llm_router.Message{Role: "user", Content: formatSteerInjection(pending)})
			events <- agenttypes.Event{Type: agenttypes.EventUserInjected, Text: raw}
		}
		if firstAttempt {
			firstAttempt = false
		} else if !compactFailed && lastInputTokens >= compact.CheckThreshold(data.Agent.Name()) {
			compactHistory()
		}
		assembled := compact.AssembleMessages(session)
		sendStart := time.Now()
		sendCtx, cancelSend := context.WithTimeout(execCtx, time.Duration(configs.AGENT_SEND_TIMEOUT_SEC)*time.Second)
		sendAgent := data.Agent
		resultCh := make(chan sendOutcome, 1)
		sendDone := make(chan struct{})
		go func() {
			defer close(sendDone)
			r, c, textEmitted, reasoned, e := streamSend(sendCtx, sendAgent, assembled, exec.Tools, reasoning, fast.Mode(sendCtx), events, &shownReasoning)
			resultCh <- sendOutcome{resp: r, code: c, err: e, textEmitted: textEmitted, reasoned: reasoned}
		}()

		stopSend := func() {
			cancelSend()
			select {
			case <-sendDone:
			case <-time.After(sendStopGrace):
				slog.Warn("stream did not stop after cancel; abandoning it",
					slog.String("model", sendAgent.Name()),
					slog.String("grace", sendStopGrace.String()))
			}
		}

		var resp *go_llm_router.Output
		var sendCode int
		var err error
		var textEmitted bool
		var reasoned bool
	waitSend:
		for {
			select {
			case <-execCtx.Done():
				stopSend()
				events <- agenttypes.Event{Type: agenttypes.EventCanceled, Model: data.Agent.Name(), Duration: time.Since(execStart)}
				if errors.Is(context.Cause(execCtx), runtime.ErrUserCanceled) {
					interactive.DeletePending(session.ID, exec.PendingTask)
				}
				return execCtx.Err()
			case out := <-resultCh:
				resp, sendCode, err, textEmitted, reasoned = out.resp, out.code, out.err, out.textEmitted, out.reasoned
				break waitSend
			}
		}
		sendDur := time.Since(sendStart)
		sendElapsed := sendDur.Round(time.Second)
		sendCtxErr := sendCtx.Err()
		stopSend()
		if err != nil {
			if execCtx.Err() != nil {
				events <- agenttypes.Event{Type: agenttypes.EventCanceled, Model: data.Agent.Name(), Duration: time.Since(execStart)}
				if errors.Is(context.Cause(execCtx), runtime.ErrUserCanceled) {
					interactive.DeletePending(session.ID, exec.PendingTask)
				}
				return execCtx.Err()
			}
			isTimeout := isSendTimeoutError(err, sendCtxErr)
			modelName := data.Agent.Name()

			reason, retryWait := retryHandler.Handle(modelName, err, sendCode, rateLimitRetryCount)
			if reason != "" {
				slog.Debug("data.Agent.Send "+reason+", model cooldown registered",
					slog.String("session", session.ID),
					slog.String("name", modelName))
			}

			if compact.IsContextLengthError(err) {
				if len(session.ContextHistories) == 0 && len(session.ToolHistories) == 0 {
					slog.Error("data.Agent.Send context length exceeded, nothing left to trim",
						slog.String("session", session.ID),
						slog.String("error", err.Error()),
						slog.Int("attempts", sendFailCount))
					msg := fmt.Sprintf("upstream %s context exceeded and nothing left to trim. Start a new session or switch to a larger-context model.", modelName)
					sendText(events, msg)
					emitChangedFiles()
					events <- agenttypes.Event{
						Type:     agenttypes.EventDone,
						Model:    modelName,
						Usage:    &usage,
						Duration: time.Since(execStart),
					}
					interactive.FinalizePending(session.ID, exec.PendingTask, msg)
					return utilsmsg.New(utilsmsg.Limit, "data.Agent.Send", "context exceeded, nothing left to trim: %w", err)
				}
				sendFailCount++
				if !compactFailed {
					compactHistory()
					if !compactFailed {
						slog.Warn("data.Agent.Send context length exceeded, compacted history",
							slog.String("session", session.ID),
							slog.Int("attempts", sendFailCount))
						continue
					}
				}
				compact.TrimFallback(&session.ContextHistories, &session.ToolHistories)
				slog.Warn("data.Agent.Send context length exceeded, trimming oldest exchange",
					slog.String("session", session.ID),
					slog.Int("attempts", sendFailCount))
				continue
			}

			slog.Debug("data.Agent.Send",
				slog.String("session", session.ID),
				slog.String("error", err.Error()),
				slog.Bool("timeout", isTimeout))

			if isTimeout && timeoutRetryCount < configs.MAX_RETRY_TIMES {
				timeoutRetryCount++
				slog.Debug("data.Agent.Send timed out, retrying same model",
					slog.String("session", session.ID),
					slog.String("name", modelName),
					slog.Int("attempt", timeoutRetryCount+1))
				select {
				case <-execCtx.Done():
					events <- agenttypes.Event{Type: agenttypes.EventCanceled, Model: data.Agent.Name(), Duration: time.Since(execStart)}
					if errors.Is(context.Cause(execCtx), runtime.ErrUserCanceled) {
						interactive.DeletePending(session.ID, exec.PendingTask)
					}
					return execCtx.Err()
				case <-time.After(configs.SEND_TIMEOUT_RETRY_INTERVAL):
				}
				continue
			}

			if retryWait > 0 {
				rateLimitRetryCount++
				slog.Debug("data.Agent.Send rate limited, retrying same model",
					slog.String("session", session.ID),
					slog.String("name", modelName),
					slog.Int("attempt", rateLimitRetryCount),
					slog.Duration("wait", retryWait))
				select {
				case <-execCtx.Done():
					events <- agenttypes.Event{Type: agenttypes.EventCanceled, Model: data.Agent.Name(), Duration: time.Since(execStart)}
					if errors.Is(context.Cause(execCtx), runtime.ErrUserCanceled) {
						interactive.DeletePending(session.ID, exec.PendingTask)
					}
					return execCtx.Err()
				case <-time.After(retryWait):
				}
				continue
			}

			next, nextName := execmodel.Next(execCtx, session.ID, modelName, &retryState)
			if next != nil {
				slog.Debug("data.Agent.Send failed, switching model",
					slog.String("session", session.ID),
					slog.String("from", modelName),
					slog.String("to", nextName))
				events <- agenttypes.Event{
					Type:      agenttypes.EventAgentResult,
					Text:      nextName,
					Model:     nextName,
					Reasoning: reasoningLabel,
				}
				data.Agent = next
				events <- agenttypes.Event{Type: agenttypes.EventCompact, Text: "tool_call"}
				if !compact.RawToolFallback(session, exec.PendingTask) {
					session.ToolHistories = nil
				}
				alreadyCall = make(map[string]string)
				sendFailCount = 0
				timeoutRetryCount = 0
				rateLimitRetryCount = 0
				emptyCount = 0
				compactFailed = false
				continue
			}

			var userMsg string
			if isTimeout {
				userMsg = fmt.Sprintf("upstream %s timed out (%s) and no healthy fallback model is available.", modelName, sendElapsed)
			} else {
				userMsg = fmt.Sprintf("upstream %s failed and no healthy fallback model is available: %s", modelName, err.Error())
			}
			slog.Error("data.Agent.Send failed, no fallback",
				slog.String("session", session.ID),
				slog.String("error", err.Error()))
			sendText(events, userMsg)
			emitChangedFiles()
			events <- agenttypes.Event{
				Type:          agenttypes.EventDone,
				Model:         modelName,
				Usage:         &usage,
				Duration:      time.Since(execStart),
				OutputElapsed: sendElapsedTotal,
			}
			interactive.FinalizePending(session.ID, exec.PendingTask, userMsg)
			return utilsmsg.Wrap(utilsmsg.Upstream, "data.Agent.Send", "", err)
		}
		retryHandler.Clear(data.Agent.Name())
		sendFailCount = 0
		timeoutRetryCount = 0
		rateLimitRetryCount = 0

		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		usage.CacheCreate += resp.Usage.CacheCreate
		usage.CacheRead += resp.Usage.CacheRead
		lastInputTokens = resp.Usage.Input + resp.Usage.CacheRead
		sendElapsedTotal += sendDur

		prov, model, _ := strings.Cut(data.Agent.Name(), "@")
		usagelog.Append(session.ID, prov, model, resp.Usage, sendDur, responseToolCalls(resp))
		if err := torii.DB(torii.DBToolCache).Set(ctx, execmodel.BetaLastModelKey+session.ID, data.Agent.Name()+"/"+reasoningLabel, torii.TTL(execmodel.BetaLastModelTTL(data.Agent.Name()))); err != nil {
			slog.Debug("torii.Set",
				slog.String("session", session.ID),
				slog.String("error", err.Error()))
		}

		usageSnapshot := usage
		events <- agenttypes.Event{
			Type:          agenttypes.EventUsageUpdate,
			Usage:         &usageSnapshot,
			ContextTokens: lastInputTokens,
		}

		if len(resp.Choices) == 0 {
			if emptyRetryExhausted(&emptyCount, events, session.ID, exec.PendingTask, data.Agent.Name(), "no choices", &usage, execStart, sendElapsedTotal) {
				return nil
			}
			continue
		}

		choice := resp.Choices[0]
		if choice.Message.ReasoningContent == "" {
			if s, ok := choice.Message.Content.(string); ok {
				if think, rest := extractThinkTag(s); think != "" {
					choice.Message.ReasoningContent = think
					choice.Message.Content = rest
				}
			}
		}

		if reasoned {
			markReasoningShown(choice.Message.ReasoningContent, &shownReasoning)
		} else {
			emitReasoning(events, choice.Message.ReasoningContent, &shownReasoning)
		}
		choice.Message.ReasoningContent = ""

		if len(choice.Message.ToolCalls) > 0 {
			emptyCount = 0
			if text, ok := choice.Message.Content.(string); ok {
				if stripped := Response(text); stripped != "" && !strings.Contains(stripped, configs.BAN_TAG) {
					if textEmitted {
						events <- agenttypes.Event{Type: agenttypes.EventTextDone}
					} else {
						sendText(events, stripped)
					}
				}
			}
			if len(clientTools) > 0 {
				var handoff []go_llm_router.ToolCall
				for _, call := range choice.Message.ToolCalls {
					if clientTools[strings.TrimSpace(call.Function.Name)] {
						handoff = append(handoff, call)
					}
				}
				if len(handoff) > 0 {
					events <- agenttypes.Event{Type: agenttypes.EventClientToolCall, ClientToolCalls: handoff}
					emitChangedFiles()
					events <- agenttypes.Event{
						Type:     agenttypes.EventDone,
						Model:    data.Agent.Name(),
						Usage:    &usage,
						Duration: time.Since(execStart),
					}
					keepPending = false
					return nil
				}
			}

			session, alreadyCall, err = toolCall(execCtx, exec, choice, session, events, allowAll, alreadyCall, &turnAllowAll)
			if err != nil {
				if errors.Is(err, ErrAskUserInterrupted) {
					return nil
				}
				return err
			}
			continue
		}

		switch value := choice.Message.Content.(type) {
		case string:
			str := value
			if str == "" {
				if emptyRetryExhausted(&emptyCount, events, session.ID, exec.PendingTask, data.Agent.Name(), "empty content", &usage, execStart, sendElapsedTotal) {
					return nil
				}
				continue
			}

			stripped := Response(str)
			if stripped == "" {
				if emptyRetryExhausted(&emptyCount, events, session.ID, exec.PendingTask, data.Agent.Name(), "content stripped to empty", &usage, execStart, sendElapsedTotal) {
					return nil
				}
				continue
			}
			emptyCount = 0

			if strings.Contains(stripped, configs.BAN_TAG) {
				refusal := guardrailRefusal(session.ID, data.Agent.Name(), stripped)
				sendText(events, refusal)
				emitChangedFiles()
				events <- agenttypes.DoneEvent(data.Agent.Name(), &usage, time.Since(execStart), sendElapsedTotal)
				interactive.FinalizePending(session.ID, exec.PendingTask, refusal)
				keepPending = false
				return nil
			}

			responseText := stripped
			if textEmitted {
				events <- agenttypes.Event{Type: agenttypes.EventTextDone}
			} else {
				sendText(events, responseText)
			}

			choice.Message.Content = sessionhistory.WithPrefix(
				sessionhistory.Record{SendAt: time.Now().UnixNano()}.Prefix(),
				stripped,
			)
			session.ToolHistories = append(session.ToolHistories, choice.Message)

			if err := saveNewHistory(execCtx, choice, session); err != nil {
				slog.Warn("writeHistory",
					slog.String("session", session.ID),
					slog.String("error", err.Error()))
			}

			interactive.FinalizePending(session.ID, exec.PendingTask, responseText)

			if pending := getSteer(session.ID); len(pending) > 0 {
				session.ToolHistories = append(session.ToolHistories, go_llm_router.Message{Role: "user", Content: formatSteerInjection(pending)})
				events <- agenttypes.Event{Type: agenttypes.EventUserInjected, Text: strings.Join(pending, "\n")}
				continue
			}

		case nil:
			if emptyRetryExhausted(&emptyCount, events, session.ID, exec.PendingTask, data.Agent.Name(), "nil content", &usage, execStart, sendElapsedTotal) {
				return nil
			}
			continue

		default:
			return utilsmsg.New(utilsmsg.Upstream, "content type", "unexpected %T", choice.Message.Content)
		}

		emitChangedFiles()
		events <- agenttypes.DoneEvent(data.Agent.Name(), &usage, time.Since(execStart), sendElapsedTotal)

		keepPending = false
		return nil
	}

	assembled := compact.AssembleMessages(session)
	summaryMessages := append(assembled, go_llm_router.Message{
		Role:    "user",
		Content: strings.TrimSpace(configs.InjectToolSummary),
	})
	summaryStart := time.Now()
	resp, _, err := data.Agent.Send(execCtx, summaryMessages, nil, reasoning, fast.Mode(execCtx))
	summaryDur := time.Since(summaryStart)
	if err == nil {
		retryHandler.Clear(data.Agent.Name())
	}
	if err == nil && len(resp.Choices) > 0 {
		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		usage.CacheCreate += resp.Usage.CacheCreate
		usage.CacheRead += resp.Usage.CacheRead
		sendElapsedTotal += summaryDur

		prov, model, _ := strings.Cut(data.Agent.Name(), "@")
		usagelog.Append(session.ID, prov, model, resp.Usage, summaryDur, responseToolCalls(resp))

		emitReasoning(events, resp.Choices[0].Message.ReasoningContent, &shownReasoning)
		if text, ok := resp.Choices[0].Message.Content.(string); ok && text != "" {
			summaryStripped := Response(text)
			if strings.Contains(summaryStripped, configs.BAN_TAG) {
				refusal := guardrailRefusal(session.ID, data.Agent.Name(), summaryStripped)
				sendText(events, refusal)
				emitChangedFiles()
				events <- agenttypes.DoneEvent(data.Agent.Name(), &usage, time.Since(execStart), sendElapsedTotal)
				interactive.FinalizePending(session.ID, exec.PendingTask, refusal)
				keepPending = false
				return nil
			}
			sendText(events, summaryStripped)
			emitChangedFiles()
			events <- agenttypes.DoneEvent(data.Agent.Name(), &usage, time.Since(execStart), sendElapsedTotal)
			interactive.FinalizePending(session.ID, exec.PendingTask, summaryStripped)
			keepPending = false
			return nil
		}
	}

	slog.Error("tool loop exhausted without a usable final answer",
		slog.String("session", session.ID),
		slog.String("name", data.Agent.Name()))
	sendEmptyData(events, session.ID, exec.PendingTask, data.Agent.Name(), &usage, execStart, sendElapsedTotal)
	return nil
}

func responseToolCalls(resp *go_llm_router.Output) []go_llm_router.ToolCall {
	if resp == nil || len(resp.Choices) == 0 {
		return nil
	}
	return resp.Choices[0].Message.ToolCalls
}

func ResolveReasoning(sessionID, name string) go_llm_router.Reasoning {
	if name == "" {
		_, name = configbot.GetModel(sessionID)
	}
	reasoning, _ := go_llm_router.ParseReasoning(name)
	return reasoning
}

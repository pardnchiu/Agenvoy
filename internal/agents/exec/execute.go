package exec

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	go_pkg_keychain "github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/agents"
	allowSkill "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/skill"
	allowTool "github.com/pardnchiu/agenvoy/internal/agents/exec/allow/tool"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/record"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configStatus "github.com/pardnchiu/agenvoy/internal/session/config/status"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	poisonRefusal     = "無法執行此操作"
	guardrailSentinel = "[KARAPPO]"
)

var (
	timestampHeaderRegex   = regexp.MustCompile(`(?m)^-{3,}\n.*\n-{3,}\n`)
	summaryLeakMarkerRegex = regexp.MustCompile(`(?i)(?:Prior Conversation Context|Prior summary|background summary of prior discussion|Strict rules:|"key_decisions"\s*:\s*\[|"current_discussion"\s*:\s*\{)`)
)

func isGuardrailRefusal(content string) bool {
	return strings.Contains(content, guardrailSentinel)
}

func StripModelResponse(str string) string {
	str = timestampHeaderRegex.ReplaceAllString(str, "")
	if loc := summaryLeakMarkerRegex.FindStringIndex(str); loc != nil {
		dropped := strings.TrimSpace(str[loc[0]:])
		head := dropped
		if len(head) > 120 {
			head = head[:120]
		}
		str = strings.TrimRight(str[:loc[0]], " \t\n\r#")
		slog.Warn("StripModelResponse summary leak stripped",
			slog.Int("dropped_chars", len(dropped)),
			slog.String("dropped_head", head))
	}
	lines := strings.Split(str, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}
		if !inFence {
			lines[i] = trimmed
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isSendTimeoutError(err error, sendCtxErr error) bool {
	if errors.Is(sendCtxErr, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "Client.Timeout"):
		return true
	case strings.Contains(s, "context deadline exceeded"):
		return true
	case strings.Contains(s, "timeout awaiting response headers"):
		return true
	case strings.Contains(s, "TLS handshake timeout"):
		return true
	case strings.Contains(s, "i/o timeout"):
		return true
	}
	return false
}

type ExecData struct {
	Agent             agentTypes.Agent
	FallbackAgents    []agentTypes.Agent
	WorkDir           string
	Skill             *skill.Skill
	SkillScanner      *runtime.SkillScanner
	Content           string
	SessionID         string
	ImageInputs       []string
	FileInputs        []string
	ExcludeTools      []string
	ExcludeSkills     []string
	ExtraSystemPrompt string
	AllowAll          bool
	PendingTask       string
}

type (
	allowAllCtxKey    struct{}
	allowListRulesKey struct{}
	parentEventsKey   struct{}
	parentWorkDirKey  struct{}
)

func Execute(ctx context.Context, data ExecData, session *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool) error {
	executeStart := time.Now()

	usedSkills := make(map[string]*skill.Skill)
	var execTrace []execStep
	if data.Skill != nil {
		usedSkills[data.Skill.Name] = data.Skill
	}
	defer func() {
		if len(usedSkills) == 0 {
			return
		}
		hasError := false
		for _, step := range execTrace {
			if step.Error != "" {
				hasError = true
				break
			}
		}
		if !hasError {
			return
		}
		trace := make([]execStep, len(execTrace))
		copy(trace, execTrace)
		for _, s := range usedSkills {
			postSkillImprove(s, trace)
		}
	}()

	if !allowAll {
		if data.Skill != nil && strings.TrimSpace(data.Skill.Content) != "" && allowSkill.Match(data.WorkDir, data.Skill.Name) {
			allowAll = true
		}
	}

	ctx = context.WithValue(ctx, allowAllCtxKey{}, allowAll)

	if !allowAll {
		ctx = context.WithValue(ctx, allowListRulesKey{}, allowTool.List(data.WorkDir))
	}

	if events != nil {
		ctx = context.WithValue(ctx, parentEventsKey{}, events)
	}

	if strings.TrimSpace(data.WorkDir) != "" {
		ctx = context.WithValue(ctx, parentWorkDirKey{}, data.WorkDir)
	}

	if session != nil && session.ID != "" {
		if err := sessionManager.AddConcurrent(ctx, session.ID); err != nil {
			return fmt.Errorf("EnterConcurrent: %w", err)
		}
		defer sessionManager.RemoveConcurrent(session.ID)

		var inputText string
		if s, ok := session.UserInput.Content.(string); ok {
			inputText = s
		}
		taskID := configStatus.Online(session.ID, inputText)
		defer configStatus.Idle(session.ID, taskID)

		original := events
		teed := make(chan agentTypes.Event, 64)
		done := make(chan struct{})
		sid := session.ID
		pushHook, hasPush := lookupPushHook(sid)
		pushCtx := ctx
		isDcPush := hasPush && !isDcPushSuppressed(pushCtx)
		var pushTextBuf strings.Builder
		var pushDoneEv agentTypes.Event
		stateless := session.Stateless
		go func() {
			defer close(done)
			for ev := range teed {
				if !stateless {
					sessionLog.Record(sid, ev)
				}
				if isDcPush {
					switch ev.Type {
					case agentTypes.EventText:
						if ev.Source == "" && ev.Text != "" {
							if pushTextBuf.Len() > 0 {
								pushTextBuf.WriteByte('\n')
							}
							pushTextBuf.WriteString(ev.Text)
						}
					case agentTypes.EventDone:
						if ev.Source == "" {
							pushDoneEv = ev
						}
					}
				}
				original <- ev
			}
		}()
		defer func() {
			close(teed)
			<-done
			if isDcPush {
				text := strings.TrimSpace(pushTextBuf.String())
				if text != "" {
					pushHook(pushCtx, PushPayload{
						SessionID: sid,
						Text:      text,
						Model:     pushDoneEv.Model,
						Usage:     pushDoneEv.Usage,
						Duration:  pushDoneEv.Duration,
						Prefix:    dcPushPrefix(pushCtx),
					})
				}
			}
		}()
		events = teed
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
		return fmt.Errorf("tools.NewExecutor: %w", err)
	}

	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()
	ctx = execCtx
	exec.CancelExecution = execCancel

	keepPending := true
	if !session.Stateless && session.ID != "" {
		if data.PendingTask != "" {
			exec.PendingTask = data.PendingTask
		} else {
			objective := data.Content
			if objective == "" {
				if s, ok := session.UserInput.Content.(string); ok {
					objective = s
				}
			}
			exec.PendingTask = interactive.CreateExecPending(session.ID, objective)
		}
		defer func() {
			if !keepPending {
				interactive.CleanupPending(session.ID, exec.PendingTask)
			}
		}()
	}

	if data.Skill != nil {
		assignBindingSkill(session, data.Skill)
	}

	if !go_pkg_filesystem_reader.Exists(filesystem.KuradbEndpointPath) {
		data.ExcludeTools = append(data.ExcludeTools,
			"list_rag", "search_rag")
	}
	cfg, _ := config.Load()
	if go_pkg_keychain.Get("agenvoy.codex.token") == "" || cfg == nil || !cfg.EnableImage2 {
		data.ExcludeTools = append(data.ExcludeTools, "generate_image")
	}
	if go_pkg_keychain.Get("GEMINI_API_KEY") == "" {
		data.ExcludeTools = append(data.ExcludeTools, "transcribe_media")
	}
	if (cfg == nil || !cfg.TelegramEnabled || go_pkg_keychain.Get("TELEGRAM_TOKEN") == "") &&
		(cfg == nil || !cfg.DiscordEnabled || go_pkg_keychain.Get("DISCORD_TOKEN") == "") {
		data.ExcludeTools = append(data.ExcludeTools,
			"format_chatbot", "list_chatbot", "send_to_chatbot")
	}

	if len(data.ExcludeTools) > 0 {
		excluded := make(map[string]bool, len(data.ExcludeTools))
		for _, name := range data.ExcludeTools {
			excluded[name] = true
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
			delete(exec.StubTools, name)
		}
	}

	limit := filesystem.MaxSkillIterations

	allAgents := make([]agentTypes.Agent, 0, 1+len(data.FallbackAgents))
	allAgents = append(allAgents, data.Agent)
	allAgents = append(allAgents, data.FallbackAgents...)
	fallbackRound := 0

	var usage agentTypes.Usage
	alreadyCall := make(map[string]string)
	turnAllowAll := false
	emptyCount := 0
	trimmedToolCalls := false
	compactedToolCalls := false
	compactFailed := false
	lastInputTokens := 0
	type sendOutcome struct {
		resp *agentTypes.Output
		err  error
	}
	sendFailCount := 0
	for range limit {
		if ctx.Err() != nil {
			keepPending = false
			return ctx.Err()
		}
		if !compactFailed && lastInputTokens >= execCompactTokenThreshold {
			if compactExec(ctx, data.Agent, session, &usage) {
				compactedToolCalls = true
				lastInputTokens = 0
			} else {
				compactFailed = true
			}
		}
		assembled := assembleMessages(session.SystemPrompts, session.OldHistories, session.SummaryMessage, session.UserInput, session.ToolHistories)
		sendStart := time.Now()
		sendCtx, cancelSend := context.WithTimeout(ctx, time.Duration(filesystem.AgentSendTimeoutSec)*time.Second)
		sendAgent := data.Agent
		resultCh := make(chan sendOutcome, 1)
		go func() {
			r, e := sendAgent.Send(sendCtx, assembled, exec.Tools)
			resultCh <- sendOutcome{resp: r, err: e}
		}()

		watchdog := time.NewTicker(UnresponsiveProbeInterval)
		var resp *agentTypes.Output
		var err error
		switched := false
	waitSend:
		for {
			select {
			case <-ctx.Done():
				watchdog.Stop()
				cancelSend()
				keepPending = false
				return ctx.Err()
			case out := <-resultCh:
				resp, err = out.resp, out.err
				break waitSend
			case <-watchdog.C:
				if utils.CheckAgentEndpointAlive(ctx, data.Agent, HealthCheckTimeout) {
					continue
				}
				next, nextName := nextAgent(ctx, &data.FallbackAgents, allAgents, &fallbackRound)
				if next == nil {
					watchdog.Stop()
					cancelSend()
					deadName := data.Agent.Name()
					slog.Error("agent unresponsive, no healthy fallback; aborting",
						slog.String("session", session.ID),
						slog.String("name", deadName))
					sendText(events, fmt.Sprintf("upstream %s is unresponsive and no healthy fallback model is available.", deadName))
					events <- agentTypes.Event{
						Type:     agentTypes.EventDone,
						Model:    deadName,
						Usage:    &usage,
						Duration: time.Since(executeStart),
					}
					return fmt.Errorf("agent %s unresponsive, no healthy fallback", deadName)
				}
				slog.Warn("agent unresponsive, switching model",
					slog.String("session", session.ID),
					slog.String("from", data.Agent.Name()),
					slog.String("to", nextName))
				events <- agentTypes.Event{
					Type:  agentTypes.EventAgentResult,
					Text:  nextName,
					Model: nextName,
				}
				data.Agent = next
				switched = true
				break waitSend
			}
		}
		watchdog.Stop()
		if switched {
			cancelSend()
			sendFailCount = 0
			continue
		}
		sendElapsed := time.Since(sendStart).Round(time.Second)
		sendCtxErr := sendCtx.Err()
		cancelSend()
		if err != nil {
			if ctx.Err() != nil {
				keepPending = false
				return ctx.Err()
			}
			isTimeout := isSendTimeoutError(err, sendCtxErr)
			modelName := data.Agent.Name()

			if isContextLengthError(err) {
				sendFailCount++
				trimmedToolCalls = trimmedToolCalls || trimOnContextExceeded(&session.OldHistories, &session.ToolHistories)
				slog.Warn("data.Agent.Send context length exceeded, trimming oldest exchange",
					slog.String("session", session.ID),
					slog.Int("attempts", sendFailCount))
				if sendFailCount >= filesystem.MaxRetry {
					slog.Error("data.Agent.Send exhausted",
						slog.String("session", session.ID),
						slog.String("error", err.Error()),
						slog.Int("attempts", sendFailCount))
					sendText(events, fmt.Sprintf("upstream %s context exceeded after %d trim attempts. Start a new session or switch to a larger-context model.", modelName, sendFailCount))
					events <- agentTypes.Event{
						Type:     agentTypes.EventDone,
						Model:    modelName,
						Usage:    &usage,
						Duration: time.Since(executeStart),
					}
					return fmt.Errorf("data.Agent.Send context exceeded after %d trims: %w", sendFailCount, err)
				}
				continue
			}

			slog.Warn("data.Agent.Send",
				slog.String("session", session.ID),
				slog.String("error", err.Error()),
				slog.Bool("timeout", isTimeout))
			next, nextName := nextAgent(ctx, &data.FallbackAgents, allAgents, &fallbackRound)
			if next != nil {
				slog.Warn("data.Agent.Send failed, switching model",
					slog.String("session", session.ID),
					slog.String("from", modelName),
					slog.String("to", nextName))
				events <- agentTypes.Event{
					Type:  agentTypes.EventAgentResult,
					Text:  nextName,
					Model: nextName,
				}
				data.Agent = next
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
			events <- agentTypes.Event{
				Type:     agentTypes.EventDone,
				Model:    modelName,
				Usage:    &usage,
				Duration: time.Since(executeStart),
			}
			return fmt.Errorf("data.Agent.Send failed: %w", err)
		}
		sendFailCount = 0

		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		usage.CacheCreate += resp.Usage.CacheCreate
		usage.CacheRead += resp.Usage.CacheRead
		lastInputTokens = resp.Usage.Input + resp.Usage.CacheRead

		if len(resp.Choices) == 0 {
			if actionError(&emptyCount, events) {
				return nil
			}
			continue
		}
		emptyCount = 0

		choice := resp.Choices[0]
		if len(choice.Message.ToolCalls) > 0 {
			toolsBefore := len(session.Tools)
			session, alreadyCall, err = toolCall(ctx, exec, choice, session, events, allowAll, alreadyCall, &turnAllowAll)
			if err != nil {
				if errors.Is(err, ErrAskUserInterrupted) {
					return nil
				}
				keepPending = false
				return err
			}
			for _, msg := range session.Tools[toolsBefore:] {
				content, _ := msg.Content.(string)
				step := execStep{Tool: extractToolName(content)}
				if strings.Contains(content, " failed: ") {
					if _, after, ok := strings.Cut(content, " failed: "); ok {
						step.Error = after
					}
				}
				if step.Tool != "" {
					execTrace = append(execTrace, step)
				}
			}
			if scanner != nil && scanner.Skills != nil {
				for _, tc := range choice.Message.ToolCalls {
					if strings.TrimSpace(tc.Function.Name) != "run_skill" {
						continue
					}
					var p struct {
						Skill string `json:"skill"`
					}
					if json.Unmarshal([]byte(tc.Function.Arguments), &p) != nil {
						continue
					}
					name := strings.TrimSpace(p.Skill)
					if name == "" {
						continue
					}
					if _, ok := usedSkills[name]; ok {
						continue
					}
					if s, ok := scanner.Skills.ByName[name]; ok && s != nil {
						usedSkills[name] = s
					}
				}
			}
			continue
		}

		switch value := choice.Message.Content.(type) {
		case string:
			str := value
			if str == "" {
				if actionError(&emptyCount, events) {
					return nil
				}
				continue
			}

			stripped := StripModelResponse(str)
			if stripped == "" {
				if actionError(&emptyCount, events) {
					return nil
				}
				continue
			}
			emptyCount = 0

			if isGuardrailRefusal(stripped) {
				sendText(events, poisonRefusal)
				events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
				interactive.FinalizePending(session.ID, exec.PendingTask, poisonRefusal)
				keepPending = false
				return nil
			}

			responseText := stripped
			if trimmedToolCalls {
				responseText += "\n\n> 因超過模型 max input，部分工具查詢資料已被裁減，建議使用更大 context window 的模型再試一次。"
			}
			if compactedToolCalls {
				responseText += "\n\n> 已自動整合壓縮工具查詢資料以維持回應品質。"
			}
			sendText(events, responseText)

			choice.Message.Content = fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), stripped)
			session.ToolHistories = append(session.ToolHistories, choice.Message)

			if err := saveNewHistory(ctx, choice, session); err != nil {
				slog.Warn("writeHistory",
					slog.String("session", session.ID),
					slog.String("error", err.Error()))
			}

			interactive.FinalizePending(session.ID, exec.PendingTask, responseText)

		case nil:
			if actionError(&emptyCount, events) {
				return nil
			}
			continue

		default:
			return fmt.Errorf("unexpected content type: %T", choice.Message.Content)
		}

		if err := record.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
			slog.Warn("usageManager.Update",
				slog.String("session", session.ID),
				slog.String("error", err.Error()))
		}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}

		keepPending = false
		return nil
	}

	assembled := assembleMessages(session.SystemPrompts, session.OldHistories, session.SummaryMessage, session.UserInput, session.ToolHistories)
	summaryMessages := append(assembled, agentTypes.Message{
		Role:    "user",
		Content: "請根據以上工具查詢結果，整理並總結回答原始問題。",
	})
	resp, err := data.Agent.Send(ctx, summaryMessages, nil)
	if err == nil && len(resp.Choices) > 0 {
		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		usage.CacheCreate += resp.Usage.CacheCreate
		usage.CacheRead += resp.Usage.CacheRead
		if text, ok := resp.Choices[0].Message.Content.(string); ok && text != "" {
			summaryStripped := StripModelResponse(text)
			if isGuardrailRefusal(summaryStripped) {
				sendText(events, poisonRefusal)
				events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
				interactive.FinalizePending(session.ID, exec.PendingTask, poisonRefusal)
				keepPending = false
				return nil
			}
			sendText(events, summaryStripped)
			if err := record.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
				slog.Warn("usageManager.Update",
					slog.String("session", session.ID),
					slog.String("error", err.Error()))
			}
			events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
			interactive.FinalizePending(session.ID, exec.PendingTask, summaryStripped)
			keepPending = false
			return nil
		}
	}

	sendText(events, "no usable data, retry later, or using other tools.")
	if err := record.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
		slog.Warn("usageManager.Update",
			slog.String("session", session.ID),
			slog.String("error", err.Error()))
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
	return nil
}

func extractToolName(content string) string {
	if len(content) < 3 || content[0] != '[' {
		return ""
	}
	end := strings.Index(content, "]")
	if end < 0 {
		return ""
	}
	return content[1:end]
}

func actionError(emptyCount *int, events chan<- agentTypes.Event) bool {
	*emptyCount++
	if *emptyCount >= filesystem.MaxEmptyResponses {
		sendText(events, "no usable data, retry later, or using other tools.")
		events <- agentTypes.Event{Type: agentTypes.EventDone}
		return true
	}
	return false
}

func sendText(events chan<- agentTypes.Event, str string) {
	str = strings.TrimRight(str, "\n")
	if str != "" {
		for line := range strings.SplitSeq(str, "\n") {
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: line}
		}
	}
	events <- agentTypes.Event{Type: agentTypes.EventTextDone}
}

func saveNewHistory(ctx context.Context, choice agentTypes.OutputChoices, session *agentTypes.AgentSession) error {
	session.Histories = append(session.Histories, choice.Message)

	if session.Stateless {
		return nil
	}

	base := min(max(session.BaseLen, 0), len(session.Histories))
	delta := make([]agentTypes.Message, 0, len(session.Histories)-base)
	for _, message := range session.Histories[base:] {
		if message.Role == "system" ||
			message.Role == "tool" ||
			(message.Role == "assistant" && len(message.ToolCalls) > 0) {
			continue
		}
		if content, ok := message.Content.(string); ok && (strings.Contains(content, poisonRefusal) || strings.Contains(content, guardrailSentinel)) {
			continue
		}
		delta = append(delta, message)
	}

	if err := sessionHistory.Append(session.ID, delta); err != nil {
		return fmt.Errorf("sessionHistory.Append: %w", err)
	}

	writeSessionHistEntry(ctx, session.ID, choice.Message)
	return nil
}

func SaveUserInputHistory(ctx context.Context, sessionID, userText string) {
	if sessionID == "" || strings.TrimSpace(userText) == "" {
		return
	}
	writeSessionHistEntry(ctx, sessionID, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	sessionLog.Append(sessionID, userText)
}

func writeSessionHistEntry(ctx context.Context, sessionID string, msg agentTypes.Message) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s:%d", sessionID, time.Now().UnixNano())
	db := torii.DB(torii.DBSessionHist)
	value := string(msgBytes)

	if setErr := db.SetVector(ctx, key, value, torii.SetDefault, nil); setErr != nil {
		if setErr = db.Set(key, value, torii.SetDefault, nil); setErr != nil {
			slog.Warn("store.DB.Set",
				slog.String("session", sessionID),
				slog.String("error", setErr.Error()))
		}
	}
}

func assignBindingSkill(session *agentTypes.AgentSession, s *skill.Skill) {
	id := "skill-assign-" + newID("skill", s.Name)
	argsJSON, _ := json.Marshal(map[string]string{"skill": s.Name})
	call := agentTypes.ToolCall{
		ID:   id,
		Type: "function",
	}
	call.Function.Name = "run_skill"
	call.Function.Arguments = string(argsJSON)

	session.ToolHistories = append(session.ToolHistories,
		agentTypes.Message{
			Role:      "assistant",
			ToolCalls: []agentTypes.ToolCall{call},
		},
		agentTypes.Message{
			Role:       "tool",
			Content:    renderActivation(s),
			ToolCallID: id,
		},
	)

	bindingHeader := fmt.Sprintf(
		"## BINDING SKILL EXECUTION — /%s\n\nThe user invoked /%s. Execute the procedure below by making the tool calls SKILL.md prescribes, in order.\n\n### How to obey\n\n- **When SKILL.md says «ask_user», invoke the `ask_user` tool** with JSON arguments matching the template SKILL.md gives. Writing a text question and waiting for a chat reply is NOT the same action and does not satisfy the step.\n- **The text following `/%s` is the user's INPUT to gather around, not a set of pre-filled answers.** Even if it looks complete, your next action is still `ask_user` to verify direction. Treat it like a topic, not a finished spec.\n- **After one tool call's result arrives, immediately make the next tool call SKILL.md prescribes**, in the same turn. Do not insert text like «下一步要不要繼續» between steps — the user already authorized the full procedure by typing `/%s`.\n- **Tool calls beat chat text.** If you find yourself writing instructions to the user («再丟一句…», «直接回我…»), stop and make the corresponding tool call instead.\n\n### Quick self-check before each turn\n\n1. What does SKILL.md say the next step is? (e.g. «呼叫 ask_user 問三維度之一»)\n2. Have I made that exact tool call in this turn? If no → make it now. If yes and result is back → make the step-after's tool call.\n\n---\n\n",
		s.Name, s.Name, s.Name, s.Name,
	)
	session.SystemPrompts = append(session.SystemPrompts, agentTypes.Message{
		Role:    "system",
		Content: bindingHeader + renderActivation(s),
	})
}

func newID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
}

func buildCrossChannelPrompt() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}
	if !cfg.TelegramEnabled && !cfg.DiscordEnabled {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Cross-channel Sending\n\n")
	sb.WriteString("When sending via `send_to_chatbot` from any session (including TUI / CLI / cron):\n\n")
	if cfg.TelegramEnabled {
		sb.WriteString("- **Telegram** (`platform=telegram`) — if the user did not name a specific chat, `list_chatbot(platform=telegram)` → `ask_user(options=[names])` → map chosen name → target_id → send. Never fabricate target_id; group ids carrying `-` prefix are especially prone to LLM hallucination and may target chats the bot was kicked from (→ 403 forbidden).\n")
		sb.WriteString("- Before composing the message argument, call `format_chatbot(platform=telegram)` (HTML mode only — markdown leaks render literally).\n")
	}
	if cfg.DiscordEnabled {
		sb.WriteString("- **Discord** (`platform=discord`) — if the user did not name a specific channel, `list_chatbot(platform=discord)` → `ask_user(options=[names])` → map chosen name → target_id → send. Never fabricate target_id.\n")
		sb.WriteString("- Before composing the message argument, call `format_chatbot(platform=discord)` (Discord markdown only — HTML / LaTeX / tables render literally).\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildExternalAgentsPrompt() string {
	agents := external.Agents()
	if len(agents) == 0 {
		return `## 外部 Agent
PATH 未偵測到任何外部 CLI binary，禁止呼叫 cross_review_with_external_agents 與 invoke_external_agent。`
	}
	return fmt.Sprintf(
		`## 外部 Agent
已偵測安裝（呼叫時仍即時驗證版本與登入）：%s
- cross_review_with_external_agents：對已產出的結果，送所有可用 agent 並行交叉審查，回傳獨立回饋供修正
- invoke_external_agent：指定單一 agent 直接生成結果

未列出的 agent 禁止使用。`,
		strings.Join(agents, "、"),
	)
}

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
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
	sessionStatus "github.com/pardnchiu/agenvoy/internal/session/status"
	"github.com/pardnchiu/agenvoy/internal/tools"
	toolSearcher "github.com/pardnchiu/agenvoy/internal/tools/searcher"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var timestampHeaderRegex = regexp.MustCompile(`(?m)^-{3,}\n.*\n-{3,}\n`)

var summaryLeakMarkerRegex = regexp.MustCompile(`(?mi)^\s*(?:[#*>\-]+\s*)?(?:Prior Conversation Context|Prior summary \(reference only\)|background summary of prior discussion|Strict rules:)`)

func StripModelResponse(text string) string {
	text = timestampHeaderRegex.ReplaceAllString(text, "")
	if loc := summaryLeakMarkerRegex.FindStringIndex(text); loc != nil {
		dropped := strings.TrimSpace(text[loc[0]:])
		head := dropped
		if len(head) > 120 {
			head = head[:120]
		}
		text = text[:loc[0]]
		slog.Warn("StripModelResponse summary leak stripped",
			slog.Int("dropped_chars", len(dropped)),
			slog.String("dropped_head", head))
	}
	lines := strings.Split(text, "\n")
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
	Skill             *filesystem.Skill
	SkillScanner      *runtime.SkillScanner
	Content           string
	SessionID         string
	ImageInputs       []string
	FileInputs        []string
	ExcludeTools      []string
	ExcludeSkills     []string
	ExtraSystemPrompt string
	AllowAll          bool
	WebMode           bool
}

type (
	allowAllCtxKey    struct{}
	allowListRulesKey struct{}
	parentEventsKey   struct{}
	parentWorkDirKey  struct{}
)

func Execute(ctx context.Context, data ExecData, session *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool) error {
	executeStart := time.Now()

	if !allowAll && data.Skill != nil && strings.TrimSpace(data.Skill.Content) != "" && allowSkill.Match(data.WorkDir, data.Skill.Name) {
		allowAll = true
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
		taskID := sessionStatus.Online(session.ID, inputText)
		defer sessionStatus.Idle(session.ID, taskID)

		original := events
		teed := make(chan agentTypes.Event, 64)
		done := make(chan struct{})
		sid := session.ID
		pushHook, hasPush := lookupPushHook(sid)
		isDcPush := hasPush && !isDcPushSuppressed(ctx)
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
					pushHook(ctx, PushPayload{
						SessionID: sid,
						Text:      text,
						Model:     pushDoneEv.Model,
						Usage:     pushDoneEv.Usage,
						Duration:  pushDoneEv.Duration,
						Prefix:    dcPushPrefix(ctx),
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

	if data.Skill != nil {
		assignBindingSkill(session, data.Skill)
	}

	if !go_pkg_filesystem_reader.Exists(filesystem.KuradbEndpointPath) {
		data.ExcludeTools = append(data.ExcludeTools,
			"rag_list_db", "rag_search_keyword", "rag_search_semantic")
	}
	if go_pkg_keychain.Get("agenvoy.codex.token") == "" {
		data.ExcludeTools = append(data.ExcludeTools, "generate_image")
	}
	if go_pkg_keychain.Get("GEMINI_API_KEY") == "" {
		data.ExcludeTools = append(data.ExcludeTools,
			"fetch_youtube_transcript", "transcribe_media")
	}
	cfg, _ := sessionManager.Load()
	if cfg == nil || !cfg.TelegramEnabled || go_pkg_keychain.Get("TELEGRAM_TOKEN") == "" {
		data.ExcludeTools = append(data.ExcludeTools,
			"telegram_format", "list_telegram_chat", "send_to_telegram_chat")
	}
	if cfg == nil || !cfg.DiscordEnabled || go_pkg_keychain.Get("DISCORD_TOKEN") == "" {
		data.ExcludeTools = append(data.ExcludeTools,
			"discord_format", "list_discord_channel", "send_to_discord_channel")
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

	var usage agentTypes.Usage
	alreadyCall := make(map[string]string)
	toolFailCount := make(map[string]int)
	emptyCount := 0
	trimmedToolCalls := false
	type sendOutcome struct {
		resp *agentTypes.Output
		err  error
	}
	sendFailCount := 0
	for range limit {
		if ctx.Err() != nil {
			return ctx.Err()
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
				return ctx.Err()
			case out := <-resultCh:
				resp, err = out.resp, out.err
				break waitSend
			case <-watchdog.C:
				if utils.CheckAgentEndpointAlive(ctx, data.Agent, HealthCheckTimeout) {
					continue
				}
				next, nextName := pickHealthyFallback(ctx, &data.FallbackAgents)
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
			next, nextName := pickHealthyFallback(ctx, &data.FallbackAgents)
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

		if len(resp.Choices) == 0 {
			if actionError(&emptyCount, events) {
				return nil
			}
			continue
		}
		emptyCount = 0

		choice := resp.Choices[0]
		if len(choice.Message.ToolCalls) > 0 {
			session, alreadyCall, err = toolCall(ctx, exec, choice, session, events, allowAll, alreadyCall, toolFailCount)
			if err != nil {
				return err
			}
			continue
		}

		switch value := choice.Message.Content.(type) {
		case string:
			text := value
			if text == "" {
				if actionError(&emptyCount, events) {
					return nil
				}
				continue
			}

			stripped := StripModelResponse(text)
			if stripped == "" {
				if actionError(&emptyCount, events) {
					return nil
				}
				continue
			}
			emptyCount = 0

			responseText := stripped
			if trimmedToolCalls {
				responseText += "\n\n> 因超過模型 max input，部分工具查詢資料已被裁減，建議使用更大 context window 的模型再試一次。"
			}
			sendText(events, responseText)

			choice.Message.Content = fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), stripped)
			session.ToolHistories = append(session.ToolHistories, choice.Message)

			if err := saveNewHistory(choice, session); err != nil {
				slog.Warn("writeHistory",
					slog.String("session", session.ID),
					slog.String("error", err.Error()))
			}

		case nil:
			if actionError(&emptyCount, events) {
				return nil
			}
			continue

		default:
			return fmt.Errorf("unexpected content type: %T", choice.Message.Content)
		}

		if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
			slog.Warn("usageManager.Update",
				slog.String("session", session.ID),
				slog.String("error", err.Error()))
		}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}

		if !session.Stateless && len(session.Tools) > 0 {
			if data, err := json.Marshal(session.Tools); err == nil {
				sessionManager.SaveToToolCall(session.ID, string(data))
			}
		}
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
			sendText(events, StripModelResponse(text))
			if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
				slog.Warn("usageManager.Update",
					slog.String("session", session.ID),
					slog.String("error", err.Error()))
			}
			events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
			return nil
		}
	}

	sendText(events, "no usable data, retry later, or using other tools.")
	if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
		slog.Warn("usageManager.Update",
			slog.String("session", session.ID),
			slog.String("error", err.Error()))
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage, Duration: time.Since(executeStart)}
	return nil
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

func sendText(events chan<- agentTypes.Event, text string) {
	text = strings.TrimRight(text, "\n")
	if text != "" {
		for line := range strings.SplitSeq(text, "\n") {
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: line}
		}
	}
	events <- agentTypes.Event{Type: agentTypes.EventTextDone}
}

func saveNewHistory(choice agentTypes.OutputChoices, session *agentTypes.AgentSession) error {
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
		delta = append(delta, message)
	}

	if err := sessionManager.AppendHistory(session.ID, delta); err != nil {
		return fmt.Errorf("sessionManager.AppendHistory: %w", err)
	}

	writeSessionHistEntry(session.ID, choice.Message)
	return nil
}

func SaveUserInputHistory(sessionID, userText string) {
	if sessionID == "" || strings.TrimSpace(userText) == "" {
		return
	}
	writeSessionHistEntry(sessionID, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	sessionLog.Append(sessionID, userText)
}

func writeSessionHistEntry(sessionID string, msg agentTypes.Message) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}
	key := fmt.Sprintf("%s:%d", sessionID, time.Now().UnixNano())
	db := torii.DB(torii.DBSessionHist)
	value := string(msgBytes)

	if setErr := db.SetVector(context.Background(), key, value, torii.SetDefault, nil); setErr != nil {
		if setErr = db.Set(key, value, torii.SetDefault, nil); setErr != nil {
			slog.Warn("store.DB.Set",
				slog.String("session", sessionID),
				slog.String("error", setErr.Error()))
		}
	}
}

func assignBindingSkill(session *agentTypes.AgentSession, s *filesystem.Skill) {
	id := "skill-assign-" + newID("skill", s.Name)
	argsJSON, _ := json.Marshal(map[string]string{"skill": s.Name})
	call := agentTypes.ToolCall{
		ID:   id,
		Type: "function",
	}
	call.Function.Name = toolSearcher.ToolName
	call.Function.Arguments = string(argsJSON)

	session.ToolHistories = append(session.ToolHistories,
		agentTypes.Message{
			Role:      "assistant",
			ToolCalls: []agentTypes.ToolCall{call},
		},
		agentTypes.Message{
			Role:       "tool",
			Content:    toolSearcher.RenderActivation(s),
			ToolCallID: id,
		},
	)

	bindingHeader := fmt.Sprintf(
		"## BINDING SKILL EXECUTION — /%s\n\nThe user invoked /%s. Execute the procedure below by making the tool calls SKILL.md prescribes, in order.\n\n### How to obey\n\n- **When SKILL.md says «ask_user», invoke the `ask_user` tool** with JSON arguments matching the template SKILL.md gives. Writing a text question and waiting for a chat reply is NOT the same action and does not satisfy the step.\n- **The text following `/%s` is the user's INPUT to gather around, not a set of pre-filled answers.** Even if it looks complete, your next action is still `ask_user` to verify direction. Treat it like a topic, not a finished spec.\n- **After one tool call's result arrives, immediately make the next tool call SKILL.md prescribes**, in the same turn. Do not insert text like «下一步要不要繼續» between steps — the user already authorized the full procedure by typing `/%s`.\n- **Tool calls beat chat text.** If you find yourself writing instructions to the user («再丟一句…», «直接回我…»), stop and make the corresponding tool call instead.\n\n### Quick self-check before each turn\n\n1. What does SKILL.md say the next step is? (e.g. «呼叫 ask_user 問三維度之一»)\n2. Have I made that exact tool call in this turn? If no → make it now. If yes and result is back → make the step-after's tool call.\n\n---\n\n",
		s.Name, s.Name, s.Name, s.Name,
	)
	session.SystemPrompts = append(session.SystemPrompts, agentTypes.Message{
		Role:    "system",
		Content: bindingHeader + toolSearcher.RenderActivation(s),
	})
}

func newID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|") + fmt.Sprint(time.Now().UnixNano())))
	return hex.EncodeToString(h[:])[:8]
}

func buildCrossChannelPrompt() string {
	cfg, err := sessionManager.Load()
	if err != nil || cfg == nil {
		return ""
	}
	if !cfg.TelegramEnabled && !cfg.DiscordEnabled {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Cross-channel Sending\n\n")
	sb.WriteString("When sending via `send_to_telegram_chat` / `send_to_discord_channel` from any session (including TUI / CLI / cron):\n\n")
	if cfg.TelegramEnabled {
		sb.WriteString("- **Telegram** — if the user did not name a specific chat, `list_telegram_chat` → `ask_user(options=[names])` → map chosen name → chat_id → send. Never fabricate chat_id; group ids carrying `-` prefix are especially prone to LLM hallucination and may target chats the bot was kicked from (→ 403 forbidden).\n")
		sb.WriteString("- Before composing the message argument, call `telegram_format` (HTML mode only — markdown leaks render literally).\n")
	}
	if cfg.DiscordEnabled {
		sb.WriteString("- **Discord** — if the user did not name a specific channel, `list_discord_channel` → `ask_user(options=[names])` → map chosen name → channel_id → send. Never fabricate channel_id.\n")
		sb.WriteString("- Before composing the message argument, call `discord_format` (Discord markdown only — HTML / LaTeX / tables render literally).\n")
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

package exec

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"time"

	go_utils_utils "github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools"
	toolSearcher "github.com/pardnchiu/agenvoy/internal/tools/searcher"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

var timestampHeaderRegex = regexp.MustCompile(`(?m)^-{3,}\n.*\n-{3,}\n`)

func StripModelResponse(text string) string {
	text = timestampHeaderRegex.ReplaceAllString(text, "")
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

var MaxToolIterations = positiveEnvInt("MAX_TOOL_ITERATIONS", 16)
var MaxSkillIterations = positiveEnvInt("MAX_SKILL_ITERATIONS", 128)
var MaxEmptyResponses = positiveEnvInt("MAX_EMPTY_RESPONSES", 8)
var MaxRetry = positiveEnvInt("MAX_SAME_PAYLOAD_RETRY", 3)

func positiveEnvInt(key string, def int) int {
	if n := go_utils_utils.GetWithDefaultInt(key, def); n > 0 {
		return n
	}
	return def
}

func hashPayload(parts ...any) string {
	h := sha256.New()
	for _, p := range parts {
		b, _ := json.Marshal(p)
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type ExecData struct {
	Agent             agentTypes.Agent
	WorkDir           string
	Skill             *skill.Skill
	SkillScanner      *skill.SkillScanner
	Content           string
	SessionID         string
	ImageInputs       []string
	FileInputs        []string
	ExcludeTools      []string
	ExtraSystemPrompt string
	AllowAll          bool
}

func Execute(ctx context.Context, data ExecData, session *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool) error {
	if session != nil && session.ID != "" {
		if err := sessionManager.AddConcurrent(ctx, session.ID); err != nil {
			return fmt.Errorf("EnterConcurrent: %w", err)
		}
		defer sessionManager.RemoveConcurrent(session.ID)

		var inputText string
		if s, ok := session.UserInput.Content.(string); ok {
			inputText = s
		}
		taskID := sessionManager.Online(session.ID, inputText)
		defer sessionManager.Idle(session.ID, taskID)

		original := events
		teed := make(chan agentTypes.Event, 64)
		done := make(chan struct{})
		sid := session.ID
		go func() {
			defer close(done)
			for ev := range teed {
				sessionManager.Record(sid, ev)
				original <- ev
			}
		}()
		defer func() {
			close(teed)
			<-done
		}()
		events = teed
	}

	// * if skill is empty, then treat as no skill
	if data.Skill != nil && data.Skill.Content == "" {
		data.Skill = nil
	}

	scanner := data.SkillScanner
	if scanner == nil {
		scanner = host.Scanner()
	}

	exec, err := tools.NewExecutor(data.WorkDir, session.ID, scanner)
	if err != nil {
		return fmt.Errorf("tools.NewExecutor: %w", err)
	}

	exec.ActiveSkill = data.Skill

	if data.Skill != nil {
		assignSkill(session, data.Skill)
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

	limit := MaxSkillIterations

	var usage agentTypes.Usage
	alreadyCall := make(map[string]string)
	toolFailCount := make(map[string]int)
	emptyCount := 0
	trimmedToolCalls := false
	var lastSendSig string
	sendFailCount := 0
	for i := 0; i < limit; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		assembled := assembleMessages(session.SystemPrompts, session.OldHistories, session.SummaryMessage, session.UserInput, session.ToolHistories)
		resp, err := data.Agent.Send(ctx, assembled, exec.Tools)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			sig := hashPayload(assembled, exec.Tools)
			if sig == lastSendSig {
				sendFailCount++
			} else {
				sendFailCount = 1
				lastSendSig = sig
			}
			if isContextLengthError(err) {
				trimmedToolCalls = trimmedToolCalls || trimOnContextExceeded(&session.OldHistories, &session.ToolHistories)
				slog.Warn("data.Agent.Send context length exceeded, trimming oldest exchange")
			} else {
				slog.Warn("data.Agent.Send",
					slog.String("error", err.Error()),
					slog.Int("sameSigCount", sendFailCount))
			}
			if sendFailCount >= MaxRetry {
				return fmt.Errorf("data.Agent.Send failed %d times with identical payload: %w", sendFailCount, err)
			}
			continue
		}
		lastSendSig = ""
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
			events <- agentTypes.Event{
				Type: agentTypes.EventText,
				Text: responseText,
			}

			choice.Message.Content = fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), stripped)
			session.ToolHistories = append(session.ToolHistories, choice.Message)

			if err := saveNewHistory(choice, session); err != nil {
				slog.Warn("writeHistory",
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
				slog.String("error", err.Error()))
		}
		events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage}

		if len(session.Tools) > 0 {
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
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: StripModelResponse(text)}
			if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
				slog.Warn("usageManager.Update",
					slog.String("error", err.Error()))
			}
			events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage}
			return nil
		}
	}

	events <- agentTypes.Event{Type: agentTypes.EventText, Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。"}
	if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output, usage.CacheCreate, usage.CacheRead); err != nil {
		slog.Warn("usageManager.Update",
			slog.String("error", err.Error()))
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage}
	return nil
}

func GetSystemPrompt(workDir string, extraSystemPrompt string, scanner *skill.SkillScanner, sessionID string, allowAll bool) string {
	systemOS := runtime.GOOS
	// var skillPath string
	// var skillExt string
	// var content string
	// if data.Skill == nil {
	// 	skillPath = "None"
	// } else {
	// 	skillPath = data.Skill.Path
	// 	skillExt = configs.SkillExecution
	// 	content = data.Skill.Content

	// 	// * add skill path, ensure path is correct
	// 	for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
	// 		resolved := filepath.Join(data.Skill.Path, prefix)

	// 		if _, err := os.Stat(resolved); err == nil {
	// 			content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
	// 		}
	// 	}
	// }
	var extraSection string
	if extra := strings.TrimSpace(extraSystemPrompt); extra != "" {
		extraSection = "---\n\n## Additional Instructions\n\n" + extra + "\n\n---\n\n"
	}

	skillsSection := ""
	if list := toolSearcher.ListBlock(scanner); list != "" {
		skillsSection = "## Skills\n\nCall `activate_skill` with one of these exact names to activate. The tool result returns the skill body + execution guidance — treat it as binding instructions for subsequent iterations. Never answer from prior knowledge when the user requests a listed skill by name.\n\n" + list
	}

	personaSection := ""
	if sessionID != "" {
		sessionManager.SaveBot(sessionID, sessionID, false)
	}
	if name, body := sessionManager.GetBot(sessionID); body != "" {
		var sb strings.Builder
		sb.WriteString("## Bot Persona\n\n")
		if name != "" {
			fmt.Fprintf(&sb, "Your operating identity for this session is `%s`. Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n", name)
		} else {
			sb.WriteString("Internalise the role description below and apply it to every reply unless an explicit user instruction overrides it.\n\n")
		}
		sb.WriteString(body)
		sb.WriteString("\n\n---\n\n")
		personaSection = sb.String()
	}

	return strings.NewReplacer(
		"{{.SystemOS}}", systemOS,
		"{{.WorkPath}}", workDir,
		// "{{.SkillPath}}", skillPath,
		// "{{.SkillExt}}", skillExt,
		// "{{.Content}}", content,
		"{{.BotPersona}}", personaSection,
		"{{.PermissionMode}}", buildPermissionModeSection(allowAll),
		"{{.AvailableSkills}}", skillsSection,
		"{{.ExternalAgents}}", buildExternalAgentsPrompt(),
		"{{.ExtraSystemPrompt}}", extraSection,
	).Replace(configs.SystemPrompt)
}

func buildPermissionModeSection(allowAll bool) string {
	if allowAll {
		return "## Permission Mode\n\n" +
			"Current mode: `always-allow` — write/exec tools auto-execute without per-call user confirmation.\n\n" +
			"For ordinary writes (`write_file` / `patch_file` / build / test / git status / git add / git commit / read-only shell), proceed directly without asking.\n\n" +
			"**Before issuing any of the following truly irreversible operations, you must call `ask_user` with a concrete description (target path / argv / DSN, why it is irreversible, blast radius) and only proceed on an explicit `yes`. A `no`, blank, or ambiguous answer means abandon this approach and pivot:**\n\n" +
			"1. Filesystem irreversible delete: `rm -rf` / `rm -r`, deleting whole directories, deleting existing files not produced by the current task\n" +
			"2. Database destruction: `DROP DATABASE` / `DROP TABLE` / `TRUNCATE`, `DELETE` / `UPDATE` without `WHERE`, any production DSN\n" +
			"3. Version control irreversible: `git reset --hard`, `git push --force` / `--force-with-lease` to main/master, deleting shared branches, `git clean -fdx`\n" +
			"4. System permission / global config: `chmod 777` / `chown -R`, edits under `/etc` / `/usr` / `/System`, launchctl / systemd unit changes, sudo escalation\n" +
			"5. Overwriting existing important user artifacts: `write_file` overwriting an existing non-empty file that has not been read this session, overwriting `.env` / credentials / lock files / `.git/index`\n" +
			"6. Cloud / infra deletion: `gcloud … delete`, `aws … delete`, `kubectl delete`, `terraform destroy`\n" +
			"7. Irreversible process operations: `shutdown` / `reboot`, `kill -9` on system service PIDs\n\n" +
			"Skipping the `ask_user` confirmation for the categories above is a violation. Ordinary file edits and routine commands are not subject to this gate."
	}
	return "## Permission Mode\n\n" +
		"Current mode: `single-confirm` — every write/exec tool call is individually confirmed by the user before it runs.\n\n" +
		"Issue tool calls as normal; do not pre-ask the user for permission in text — the harness already handles confirmation per-call. Treat a denied tool call as a directive to pivot: the user has rejected this exact approach, do not retry the same shape."
}

func actionError(emptyCount *int, events chan<- agentTypes.Event) bool {
	*emptyCount++
	if *emptyCount >= MaxEmptyResponses {
		events <- agentTypes.Event{
			Type: agentTypes.EventText,
			Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。",
		}
		events <- agentTypes.Event{Type: agentTypes.EventDone}
		return true
	}
	return false
}

func saveNewHistory(choice agentTypes.OutputChoices, session *agentTypes.AgentSession) error {
	session.Histories = append(session.Histories, choice.Message)

	newHistories := make([]agentTypes.Message, 0, len(session.Histories))
	for _, message := range session.Histories {
		if message.Role == "system" ||
			message.Role == "tool" ||
			(message.Role == "assistant" && len(message.ToolCalls) > 0) {
			continue
		}
		newHistories = append(newHistories, message)
	}

	historyBytes, err := json.Marshal(newHistories)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	if err = sessionManager.SaveHistory(session.ID, string(historyBytes)); err != nil {
		return fmt.Errorf("sessionManager.SaveHistory: %w", err)
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
	sessionManager.AppendActionUserInput(sessionID, userText)
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
				slog.String("error", setErr.Error()))
		}
	}
}

func assignSkill(session *agentTypes.AgentSession, s *skill.Skill) {
	id := "skill-assign-" + utils.NewID("skill", s.Name)
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
}

func buildExternalAgentsPrompt() string {
	agents := external.Agents()
	if len(agents) == 0 {
		return `## 外部 Agent
目前無宣告的外部 agent，禁止呼叫 cross_review_with_external_agents 與 invoke_external_agent。`
	}
	return fmt.Sprintf(
		`## 外部 Agent
已宣告（呼叫時仍即時驗證安裝與登入）：%s
- cross_review_with_external_agents：對已產出的結果，送所有可用 agent 並行交叉審查，回傳獨立回饋供修正
- invoke_external_agent：指定單一 agent 直接生成結果

未列出的 agent 禁止使用。`,
		strings.Join(agents, "、"),
	)
}

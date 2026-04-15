package exec

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/store"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/externalAgent"
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

var MaxToolIterations = func() int {
	if v := os.Getenv("MAX_TOOL_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 16
}()

var MaxSkillIterations = func() int {
	if v := os.Getenv("MAX_SKILL_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 128
}()

var MaxEmptyResponses = func() int {
	if v := os.Getenv("MAX_EMPTY_RESPONSES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8
}()

var MaxRetry = func() int {
	if v := os.Getenv("MAX_SAME_PAYLOAD_RETRY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 3
}()

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
	Content           string
	ImageInputs       []string
	FileInputs        []string
	ExcludeTools      []string
	ExtraSystemPrompt string
}

func Execute(ctx context.Context, data ExecData, session *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool) error {
	// * if skill is empty, then treat as no skill
	if data.Skill != nil && data.Skill.Content == "" {
		data.Skill = nil
	}

	exec, err := tools.NewExecutor(data.WorkDir, session.ID)
	if err != nil {
		return fmt.Errorf("tools.NewExecutor: %w", err)
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

	limit := MaxToolIterations
	if data.Skill != nil {
		limit = MaxSkillIterations
	}

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
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
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

func GetSystemPrompt(data ExecData) string {
	systemOS := runtime.GOOS

	var skillPath string
	var skillExt string
	var content string
	if data.Skill == nil {
		skillPath = "None"
	} else {
		skillPath = data.Skill.Path
		skillExt = configs.SkillExecution
		content = data.Skill.Content

		// * add skill path, ensure path is correct
		for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
			resolved := filepath.Join(data.Skill.Path, prefix)

			if _, err := os.Stat(resolved); err == nil {
				content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
			}
		}
	}
	var extraSection string
	if extra := strings.TrimSpace(data.ExtraSystemPrompt); extra != "" {
		extraSection = "---\n\n## Additional Instructions\n\n" + extra + "\n\n---\n\n"
	}
	return strings.NewReplacer(
		"{{.SystemOS}}", systemOS,
		"{{.WorkPath}}", data.WorkDir,
		"{{.SkillPath}}", skillPath,
		"{{.SkillExt}}", skillExt,
		"{{.Content}}", content,
		"{{.ExternalAgents}}", buildExternalAgentsPrompt(),
		"{{.ExtraSystemPrompt}}", extraSection,
	).Replace(configs.SystemPrompt)
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

	msgBytes, err := json.Marshal(choice.Message)
	if err == nil {
		key := fmt.Sprintf("%s:%d", session.ID, time.Now().UnixNano())
		if setErr := store.DB(store.DBSessionHist).Set(key, string(msgBytes), store.SetDefault, nil); setErr != nil {
			slog.Warn("store.DB.Set",
				slog.String("error", setErr.Error()))
		}
	}

	return nil
}

func buildExternalAgentsPrompt() string {
	agents := externalAgent.GetAgents()
	if len(agents) == 0 {
		return `## 外部 Agent
目前無宣告的外部 agent，禁止呼叫 verify_with_external_agent 與 call_external_agent。`
	}
	return fmt.Sprintf(
		`## 外部 Agent
已宣告（呼叫時仍即時驗證安裝與登入）：%s
- verify_with_external_agent：所有可用 agent 並行驗證，回傳獨立回饋供主 agent 參考修正
- call_external_agent：指定單一 agent 直接生成結果

未列出的 agent 禁止使用。`,
		strings.Join(agents, "、"),
	)
}

package exec

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools"
)

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

type ExecData struct {
	Agent       agentTypes.Agent
	WorkDir     string
	Skill       *skill.Skill
	Content     string
	ImageInputs []string
	FileInputs  []string
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

	limit := MaxToolIterations
	if data.Skill != nil {
		limit = MaxSkillIterations
	}

	maxInputTokens := data.Agent.MaxInputTokens()

	var usage agentTypes.Usage
	alreadyCall := make(map[string]string)
	emptyCount := 0
	for i := 0; i < limit; i++ {
		if i > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		trimmed, err := trimMessages(session.Messages, maxInputTokens)
		if err != nil {
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: "訊息內容超出模型可接受的 token 上限，請縮短輸入後再試。"}
			events <- agentTypes.Event{Type: agentTypes.EventDone}
			return nil
		}
		session.Messages = trimmed
		resp, err := data.Agent.Send(ctx, session.Messages, exec.Tools)
		if err != nil {
			slog.Warn("data.Agent.Send",
				slog.String("error", err.Error()))
			continue
		}

		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output

		if len(resp.Choices) == 0 {
			if actionError(&emptyCount, events) {
				return nil
			}
			continue
		}
		emptyCount = 0

		choice := resp.Choices[0]
		if len(choice.Message.ToolCalls) > 0 {
			session, alreadyCall, err = toolCall(ctx, exec, choice, session, events, allowAll, alreadyCall)
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

			cleaned := extractSummary(session.ID, text)
			if cleaned == "" {
				if actionError(&emptyCount, events) {
					return nil
				}
				continue
			}
			emptyCount = 0

			events <- agentTypes.Event{
				Type: agentTypes.EventText,
				Text: cleaned,
			}

			choice.Message.Content = fmt.Sprintf("---\n當前時間: %s\n---\n%s", time.Now().Format("2006-01-02 15:04:05"), cleaned)
			session.Messages = append(session.Messages, choice.Message)

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

		if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output); err != nil {
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

	summaryMessages := append(session.Messages, agentTypes.Message{
		Role:    "user",
		Content: "請根據以上工具查詢結果，整理並總結回答原始問題。",
	})
	resp, err := data.Agent.Send(ctx, summaryMessages, nil)
	if err == nil && len(resp.Choices) > 0 {
		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		if text, ok := resp.Choices[0].Message.Content.(string); ok && text != "" {
			cleaned := extractSummary(session.ID, text)
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: cleaned}
			if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output); err != nil {
				slog.Warn("usageManager.Update",
					slog.String("error", err.Error()))
			}
			events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage}
			return nil
		}
	}

	events <- agentTypes.Event{Type: agentTypes.EventText, Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。"}
	if err := filesystem.UpdateUsage(data.Agent.Name(), usage.Input, usage.Output); err != nil {
		slog.Warn("usageManager.Update",
			slog.String("error", err.Error()))
	}
	events <- agentTypes.Event{Type: agentTypes.EventDone, Model: data.Agent.Name(), Usage: &usage}
	return nil
}

func GetSystemPrompt(data ExecData) string {
	systemOS := runtime.GOOS
	localtime := time.Now().Format("2006-01-02 15:04:05 MST")

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
	return strings.NewReplacer(
		"{{.SystemOS}}", systemOS,
		"{{.Localtime}}", localtime,
		"{{.WorkPath}}", data.WorkDir,
		"{{.SkillPath}}", skillPath,
		"{{.SkillExt}}", skillExt,
		"{{.Content}}", content,
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
	return nil
}

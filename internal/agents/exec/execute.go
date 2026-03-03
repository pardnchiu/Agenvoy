package exec

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

//go:embed prompt/systemPrompt.md
var systemPrompt string

//go:embed prompt/skillExtension.md
var skillExtensionPrompt string

const (
	MaxToolIterations  = 16
	MaxSkillIterations = 128
)

func Execute(ctx context.Context, agent agentTypes.Agent, workDir string, skill *skill.Skill, userInput string, events chan<- agentTypes.Event, allowAll bool) error {
	// if skill is empty, then treat as no skill
	if skill != nil && skill.Content == "" {
		skill = nil
	}

	configDir, err := utils.GetConfigDir("sessions")
	if err != nil {
		return fmt.Errorf("utils.ConfigDir: %w", err)
	}

	prompt := getSystemPrompt(workDir, skill)
	session, err := getSession(prompt, userInput)
	if err != nil {
		return fmt.Errorf("getSession: %w", err)
	}

	exec, err := tools.NewExecutor(workDir, session.ID)
	if err != nil {
		return fmt.Errorf("tools.NewExecutor: %w", err)
	}

	limit := MaxToolIterations
	if skill != nil {
		limit = MaxSkillIterations
	}

	alreadyCall := make(map[string]string)
	emptyCount := 0
	const maxEmpty = 8
	for i := 0; i < limit; i++ {
		resp, err := agent.Send(ctx, session.Messages, exec.Tools)
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			emptyCount++
			if emptyCount >= maxEmpty {
				events <- agentTypes.Event{Type: agentTypes.EventText, Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。"}
				events <- agentTypes.Event{Type: agentTypes.EventDone}
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
				text = "工具無法取得資料，請稍後再試或改用其他方式查詢。"
			}
			cleaned := extractSummary(configDir, session.ID, text)

			events <- agentTypes.Event{Type: agentTypes.EventText, Text: cleaned}

			choice.Message.Content = fmt.Sprintf("ts:%d\n%s", time.Now().Unix(), cleaned)

			session.Messages = append(session.Messages, choice.Message)

			err := writeHistory(choice, configDir, session)
			if err != nil {
				slog.Warn("Failed to write history",
					slog.String("error", err.Error()))
			}
		case nil:
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。"}
		default:
			return fmt.Errorf("unexpected content type: %T", choice.Message.Content)
		}

		events <- agentTypes.Event{Type: agentTypes.EventDone}

		if len(session.Tools) > 0 {
			now := time.Now()
			date := now.Format("2006-01-02")
			dateWithSec := now.Format("2006-01-02-15-04-05")
			toolActionsDir := filepath.Join(configDir.Home, session.ID, "tool_calls", date)
			if err := os.MkdirAll(toolActionsDir, 0755); err == nil {
				filename := dateWithSec + ".json"
				toolActionsPath := filepath.Join(toolActionsDir, filename)
				if data, err := json.Marshal(session.Tools); err == nil {
					os.WriteFile(toolActionsPath, data, 0644)
				}
			}
		}
		return nil
	}

	summaryMessages := append(session.Messages, agentTypes.Message{
		Role:    "user",
		Content: "請根據以上工具查詢結果，整理並總結回答原始問題。",
	})
	resp, err := agent.Send(ctx, summaryMessages, nil)
	if err == nil && len(resp.Choices) > 0 {
		if text, ok := resp.Choices[0].Message.Content.(string); ok && text != "" {
			cleaned := extractSummary(configDir, session.ID, text)
			events <- agentTypes.Event{Type: agentTypes.EventText, Text: cleaned}
			events <- agentTypes.Event{Type: agentTypes.EventDone}
			return nil
		}
	}

	events <- agentTypes.Event{Type: agentTypes.EventText, Text: "工具無法取得資料，請稍後再試或改用其他方式查詢。"}
	events <- agentTypes.Event{Type: agentTypes.EventDone}
	return nil
}

func getSystemPrompt(workDir string, skill *skill.Skill) string {
	if skill == nil {
		return strings.NewReplacer(
			"{{.WorkPath}}", workDir,
			"{{.SkillPath}}", "None",
			"{{.SkillExt}}", "",
			"{{.Content}}", "",
		).Replace(systemPrompt)
	}
	content := skill.Content

	for _, prefix := range []string{"scripts/", "templates/", "assets/"} {
		resolved := filepath.Join(skill.Path, prefix)

		if _, err := os.Stat(resolved); err == nil {
			content = strings.ReplaceAll(content, prefix, resolved+string(filepath.Separator))
		}
	}

	return strings.NewReplacer(
		"{{.WorkPath}}", workDir,
		"{{.SkillPath}}", skill.Path,
		"{{.SkillExt}}", skillExtensionPrompt,
		"{{.Content}}", content,
	).Replace(systemPrompt)
}

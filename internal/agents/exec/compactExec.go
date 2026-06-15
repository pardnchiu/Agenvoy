package exec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

const execCompactTokenThreshold = 64000

func compactExec(ctx context.Context, agent agentTypes.Agent, session *agentTypes.AgentSession, usage *agentTypes.Usage) bool {
	userQuestion := extractUserText(session.UserInput)
	if userQuestion == "" {
		return false
	}
	if len(session.ToolHistories) == 0 {
		return false
	}

	lastGroupIdx := -1
	for i := len(session.ToolHistories) - 1; i >= 0; i-- {
		if session.ToolHistories[i].Role == "assistant" && len(session.ToolHistories[i].ToolCalls) > 0 {
			lastGroupIdx = i
			break
		}
	}
	if lastGroupIdx <= 0 {
		return false
	}

	var sb strings.Builder
	for _, msg := range session.ToolHistories[:lastGroupIdx] {
		switch {
		case msg.Role == "assistant" && len(msg.ToolCalls) > 0:
			for _, tc := range msg.ToolCalls {
				fmt.Fprintf(&sb, "[call] %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			}
		case msg.Role == "tool":
			content, _ := msg.Content.(string)
			fmt.Fprintf(&sb, "[result] %s\n\n", content)
		case msg.Role == "assistant":
			content, _ := msg.Content.(string)
			if content != "" {
				fmt.Fprintf(&sb, "[assistant] %s\n\n", content)
			}
		case msg.Role == "user":
			content, _ := msg.Content.(string)
			if content != "" {
				fmt.Fprintf(&sb, "[context] %s\n\n", content)
			}
		}
	}
	if sb.Len() == 0 {
		return false
	}
	tail := session.ToolHistories[lastGroupIdx:]

	prompt := strings.NewReplacer(
		"{{.UserQuestion}}", userQuestion,
	).Replace(strings.TrimSpace(configs.CompactExecPrompt))

	messages := []agentTypes.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: sb.String()},
	}

	compactCtx, cancel := context.WithTimeout(ctx, time.Duration(filesystem.AgentSendTimeoutSec)*time.Second)
	defer cancel()

	resp, err := agent.Send(compactCtx, messages, nil)
	if err != nil {
		slog.Warn("compactExec agent.Send",
			slog.String("session", session.ID),
			slog.String("error", err.Error()))
		return false
	}
	if len(resp.Choices) == 0 {
		return false
	}

	if usage != nil {
		usage.Input += resp.Usage.Input
		usage.Output += resp.Usage.Output
		usage.CacheCreate += resp.Usage.CacheCreate
		usage.CacheRead += resp.Usage.CacheRead
	}

	result, ok := resp.Choices[0].Message.Content.(string)
	if !ok || strings.TrimSpace(result) == "" {
		return false
	}

	session.OldHistories = nil
	session.SummaryMessage = agentTypes.Message{}
	session.ToolHistories = append(
		[]agentTypes.Message{
			{Role: "user", Content: "以下是先前工具查詢的整合結果，請基於此資料繼續回答原始問題。"},
			{Role: "assistant", Content: strings.TrimSpace(result)},
		},
		tail...,
	)

	slog.Info("compactExec completed",
		slog.Int("input_tokens", resp.Usage.Input),
		slog.Int("output_tokens", resp.Usage.Output))
	return true
}

func extractUserText(input agentTypes.Message) string {
	switch v := input.Content.(type) {
	case string:
		return v
	case []agentTypes.ContentPart:
		for _, part := range v {
			if part.Type == "text" {
				return part.Text
			}
		}
	}
	return ""
}

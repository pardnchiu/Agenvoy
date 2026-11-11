package compact

import (
	"context"
	"fmt"
	"math"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agentsummary "github.com/pardnchiu/agenvoy/internal/agents/exec/summary"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/todo"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func ToolHistory(ctx context.Context, agent agenttypes.Agent, session *agenttypes.Session, usage *go_llm_router.Usage, taskHash string) bool {
	// * step1: check user input is exist or not
	message := extractMessage(session.UserMessage)
	if message == "" {
		return false
	}

	// * step2: check tool histories is exist or not
	if len(session.ToolHistories) == 0 {
		return false
	}

	// * step3: extract todo result and get clean tool histories
	histories := session.ToolHistories
	var planPair []go_llm_router.Message
	if taskHash != "" {
		planPair = todo.LastPair(histories)
		histories = todo.Strip(histories)
	}

	// * step4: get tool call indices
	indices := getToolCallIndices(histories)
	total := len(indices)
	if total < 3 {
		return false
	}

	// * step5: get boundary and split histories
	// * 20 of the tool call, at least 2
	size := min(max(int(math.Round(float64(total)*0.2)), 2), total-1)
	idx := indices[size]
	prefix := histories[:idx]
	tail := histories[idx:]

	// * step6: format tool histories to string
	raw := formatTool(prefix)
	if raw == "" {
		return false
	}

	// * step7: prepare conpact system prompt
	prompt := strings.NewReplacer(
		"{{.Message}}", message,
	).Replace(strings.TrimSpace(configs.CompactExecPrompt))

	messages := []go_llm_router.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: raw},
	}

	// * step8: start to compact tool histories
	result := agentsummary.Send(ctx, agent, session.ID, usage, messages, go_llm_router.ReasoningNone, CheckThreshold)
	if result == "" {
		return false
	}

	session.ContextHistories = nil
	head := []go_llm_router.Message{
		{
			Role:    "user",
			Content: "Prior tool findings (compacted). Use to answer the original question — user has not been replied to yet:\n\n" + result,
		},
	}
	head = append(head, planPair...)
	session.ToolHistories = append(head, tail...)

	return true
}

func getToolCallIndices(histories []go_llm_router.Message) []int {
	var idx []int
	for i, e := range histories {
		if e.Role == "assistant" && len(e.ToolCalls) > 0 {
			idx = append(idx, i)
		}
	}
	return idx
}

func formatTool(prefix []go_llm_router.Message) string {
	var sb strings.Builder
	for _, msg := range prefix {
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
	return sb.String()
}

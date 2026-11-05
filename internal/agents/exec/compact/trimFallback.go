package compact

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
)

var (
	overflowMu  sync.RWMutex
	overflowDic = map[string]int{}
)

var overflowNumber = regexp.MustCompile(`\d{4,}`)

func NoteOverflow(modelName string, err error) {
	limit := 0
	for _, raw := range overflowNumber.FindAllString(strings.ReplaceAll(err.Error(), ",", ""), -1) {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n < 4096 {
			continue
		}
		if limit == 0 || n < limit {
			limit = n
		}
	}
	if limit == 0 {
		return
	}
	overflowMu.Lock()
	defer overflowMu.Unlock()
	if prev, ok := overflowDic[modelName]; !ok || limit < prev {
		overflowDic[modelName] = limit
	}
}

func overflowLimit(modelName string) (int, bool) {
	overflowMu.RLock()
	defer overflowMu.RUnlock()
	limit, ok := overflowDic[modelName]
	return limit, ok
}

func TrimRatio(ctx context.Context, sessionID, objective string, oldHistory *[]go_llm_router.Message, toolCall *[]go_llm_router.Message, ratio float64) bool {
	total := messagesSize(*oldHistory) + messagesSize(*toolCall)
	if total == 0 {
		return false
	}
	target := int(float64(total) * ratio)
	pruneToolsWithDecision(ctx, sessionID, objective, toolCall)
	for messagesSize(*oldHistory)+messagesSize(*toolCall) > target {
		before := len(*oldHistory) + len(*toolCall)
		TrimFallback(oldHistory, toolCall)
		if len(*oldHistory)+len(*toolCall) == before {
			break
		}
	}
	return messagesSize(*oldHistory)+messagesSize(*toolCall) < total
}

func messagesSize(list []go_llm_router.Message) int {
	size := 0
	for _, m := range list {
		raw, err := json.Marshal(m)
		if err != nil {
			continue
		}
		size += len(raw)
	}
	return size
}

func TrimFallback(oldHistory *[]go_llm_router.Message, toolCall *[]go_llm_router.Message) {
	if len(*oldHistory) > 0 {
		n := 2
		if len(*oldHistory) < 2 {
			n = 1
		}
		*oldHistory = (*oldHistory)[n:]
		return
	}

	if len(*toolCall) == 0 {
		return
	}

	firstToolCall := -1
	for i, message := range *toolCall {
		if message.Role == "assistant" && len(message.ToolCalls) > 0 {
			firstToolCall = i
			break
		}
	}

	if firstToolCall == -1 {
		*toolCall = (*toolCall)[1:]
		return
	}

	ids := make(map[string]bool, len((*toolCall)[firstToolCall].ToolCalls))
	for _, tool := range (*toolCall)[firstToolCall].ToolCalls {
		ids[tool.ID] = true
	}

	kept := make([]go_llm_router.Message, 0, len(*toolCall))
	for i, m := range *toolCall {
		if i == firstToolCall {
			continue
		}
		if m.ToolCallID != "" && ids[m.ToolCallID] {
			continue
		}
		kept = append(kept, m)
	}
	*toolCall = kept
}

func IsContextLengthError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "prompt is too long") ||
		(strings.Contains(msg, "token count") && strings.Contains(msg, "exceeds")) ||
		strings.Contains(msg, "exceeds the maximum number of tokens") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "reduce the length of the messages") ||
		strings.Contains(msg, "too many tokens") ||
		strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "maximum prompt length") ||
		strings.Contains(msg, "input size exceeds") ||
		strings.Contains(msg, "max_tokens must be at least") ||
		strings.Contains(msg, "context length") ||
		strings.Contains(msg, "range of input length")
}

package exec

import (
	"strings"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func assembleMessages(systemPart []agentTypes.Message, oldHistory []agentTypes.Message, userInput agentTypes.Message, toolCall []agentTypes.Message) []agentTypes.Message {
	result := make([]agentTypes.Message, 0, len(systemPart)+len(oldHistory)+1+len(toolCall))
	result = append(result, systemPart...)
	result = append(result, oldHistory...)
	result = append(result, userInput)
	result = append(result, toolCall...)
	return result
}

func trimOnContextExceeded(oldHistory *[]agentTypes.Message, toolCall *[]agentTypes.Message) bool {
	if len(*oldHistory) > 0 {
		n := 2
		if len(*oldHistory) < 2 {
			n = 1
		}
		*oldHistory = (*oldHistory)[n:]
		return false
	}

	if len(*toolCall) == 0 {
		return false
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
		return false
	}

	ids := make(map[string]bool, len((*toolCall)[firstToolCall].ToolCalls))
	for _, tool := range (*toolCall)[firstToolCall].ToolCalls {
		ids[tool.ID] = true
	}

	kept := make([]agentTypes.Message, 0, len(*toolCall))
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
	return true
}

func isContextLengthError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context_length_exceeded") ||
		strings.Contains(msg, "maximum context length") ||
		strings.Contains(msg, "prompt is too long") ||
		(strings.Contains(msg, "token count") && strings.Contains(msg, "exceeds")) ||
		strings.Contains(msg, "exceeds the maximum number of tokens")
}

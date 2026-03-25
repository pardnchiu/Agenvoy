package exec

import (
	"fmt"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func calculateTokens(message agentTypes.Message) int {
	switch v := message.Content.(type) {
	case string:
		return len(v) / 4
	case []agentTypes.ContentPart:
		total := 0
		for _, part := range v {
			if part.Type == "text" {
				total += len(part.Text) / 4
			} else if part.Type == "image_url" {
				total += 1000
			}
		}
		return total
	default:
		return 0
	}
}

func trimMessages(messages []agentTypes.Message, maxTokens int) ([]agentTypes.Message, error) {
	if maxTokens <= 0 {
		return messages, nil
	}

	var systemMessages []agentTypes.Message
	var lastUser *agentTypes.Message
	var history []agentTypes.Message

	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && lastUserIndex == -1 {
			lastUserIndex = i
			break
		}
	}

	for i, message := range messages {
		switch {
		case i == lastUserIndex:
			msg := message
			lastUser = &msg
		case message.Role == "system":
			systemMessages = append(systemMessages, message)
		default:
			history = append(history, message)
		}
	}

	total := 0
	for _, msg := range messages {
		total += calculateTokens(msg)
	}
	if total <= maxTokens {
		return reorder(history, systemMessages, lastUser, false), nil
	}

	reserved := 0
	for _, m := range systemMessages {
		reserved += calculateTokens(m)
	}
	if lastUser != nil {
		reserved += calculateTokens(*lastUser)
	}

	budget := maxTokens - reserved
	if budget <= 0 {
		return nil, fmt.Errorf("single message exceeds token limit (%d tokens)", reserved)
	}

	kept := make([]agentTypes.Message, 0, len(history))
	used := 0
	for i := len(history) - 1; i >= 0; i-- {
		cost := calculateTokens(history[i])
		if used+cost > budget {
			break
		}
		used += cost
		kept = append(kept, history[i])
	}

	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}

	trimmed := len(kept) < len(history)

	if trimmed && len(kept) > 0 {
		if text, ok := kept[0].Content.(string); ok {
			kept[0].Content = fmt.Sprintf("...\n%s", text)
		}
	}

	return reorder(kept, systemMessages, lastUser, trimmed), nil
}

func reorder(history []agentTypes.Message, systemMessages []agentTypes.Message, lastUser *agentTypes.Message, trimmed bool) []agentTypes.Message {
	result := make([]agentTypes.Message, 0, len(systemMessages)+len(history)+2)

	result = append(result, systemMessages...)

	if trimmed {
		result = append(result, agentTypes.Message{
			Role:    "system",
			Content: "因內容長度超過模型上限，已自動移除較舊的對話訊息，本次回答可能缺少先前的上下文。",
		})
	}

	result = append(result, history...)

	if lastUser != nil {
		result = append(result, *lastUser)
	}

	return result
}

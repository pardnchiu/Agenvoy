package compact

import (
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func AssembleMessages(session *agenttypes.Session) []go_llm_router.Message {
	result := make([]go_llm_router.Message, 0, len(session.SystemPrompts)+len(session.ContextHistories)+2+len(session.ToolHistories))
	result = append(result, session.SystemPrompts...)
	for _, msg := range session.ContextHistories {
		if content, ok := msg.Content.(string); ok && strings.Contains(content, configs.BAN_TAG) {
			continue
		}
		result = append(result, msg)
	}
	if session.TurnContext.Role != "" {
		result = append(result, session.TurnContext)
	}
	result = append(result, session.UserMessage)
	result = append(result, session.ToolHistories...)

	return result
}

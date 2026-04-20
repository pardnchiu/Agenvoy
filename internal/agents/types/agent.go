package agentTypes

import (
	"context"

	"github.com/pardnchiu/agenvoy/internal/skill"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type Agent interface {
	Name() string
	Send(ctx context.Context, messages []Message, toolDefs []toolTypes.Tool) (*Output, error)
	Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- Event, allowAll bool) error
}

type AgentRegistry struct {
	Registry map[string]Agent
	Entries  []AgentEntry
	Fallback Agent
}

type AgentEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AgentSession struct {
	ID             string
	SystemPrompts  []Message
	OldHistories   []Message
	SummaryMessage Message
	UserInput      Message
	ToolHistories  []Message
	Tools          []Message
	Histories      []Message

	VerifyRounds    int
	VerifyFeedbacks []string
}

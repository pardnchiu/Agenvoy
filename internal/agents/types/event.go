package agentTypes

import "encoding/json"

type EventType int

func (e EventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

const (
	EventText EventType = iota
	EventSkillSelect
	EventSkillResult
	EventAgentSelect
	EventAgentResult
	EventToolCall
	EventToolCallStart
	EventToolCallText
	EventToolCallEnd
	EventToolResult
	EventToolSkipped
	EventToolConfirm
	EventExecError
	EventError
	EventSummaryGenerate
	EventDone
)

func (e EventType) String() string {
	switch e {
	case EventText:
		return "EventText"
	case EventSkillSelect:
		return "EventSkillSelect"
	case EventSkillResult:
		return "EventSkillResult"
	case EventAgentSelect:
		return "EventAgentSelect"
	case EventAgentResult:
		return "EventAgentResult"
	case EventToolCall:
		return "EventToolCall"
	case EventToolCallStart:
		return "EventToolCallStart"
	case EventToolCallText:
		return "EventToolCallText"
	case EventToolCallEnd:
		return "EventToolCallEnd"
	case EventToolResult:
		return "EventToolResult"
	case EventToolSkipped:
		return "EventToolSkipped"
	case EventToolConfirm:
		return "EventToolConfirm"
	case EventExecError:
		return "EventExecError"
	case EventError:
		return "EventError"
	case EventSummaryGenerate:
		return "EventSummaryGenerate"
	case EventDone:
		return "EventDone"
	default:
		return "EventUnknown"
	}
}

type Event struct {
	Type     EventType `json:"type"`
	Text     string    `json:"text,omitempty"`
	ToolName string    `json:"tool_name,omitempty"`
	ToolArgs string    `json:"tool_args,omitempty"`
	ToolID   string    `json:"tool_id,omitempty"`
	Result   string    `json:"result,omitempty"`
	Model    string    `json:"model,omitempty"`
	Usage    *Usage    `json:"usage,omitempty"`
	Err      error     `json:"-"`
	ReplyCh  chan bool `json:"-"`
}

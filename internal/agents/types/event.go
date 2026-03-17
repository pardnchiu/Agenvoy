package agentTypes

type EventType int

const (
	EventText EventType = iota
	EventAgentSelect
	EventAgentResult
	EventSkillSelect
	EventSkillResult
	EventToolCall
	EventToolCallStart
	EventToolCallText
	EventToolCallEnd
	EventToolResult
	EventToolSkipped
	EventToolConfirm
	EventExecError
	EventError
	EventDone
)

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

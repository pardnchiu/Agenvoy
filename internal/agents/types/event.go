package types

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
)

type EventType int

func (e EventType) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

var eventTypeByName = map[string]EventType{
	"EventText":            EventText,
	"EventTextDelta":       EventTextDelta,
	"EventTextDone":        EventTextDone,
	"EventSkillResult":     EventSkillResult,
	"EventAgentSelect":     EventAgentSelect,
	"EventAgentResult":     EventAgentResult,
	"EventToolCall":        EventToolCall,
	"EventToolCallStart":   EventToolCallStart,
	"EventToolCallText":    EventToolCallText,
	"EventToolCallEnd":     EventToolCallEnd,
	"EventToolProgress":    EventToolProgress,
	"EventToolResult":      EventToolResult,
	"EventToolSkipped":     EventToolSkipped,
	"EventToolConfirm":     EventToolConfirm,
	"EventExecError":       EventExecError,
	"EventError":           EventError,
	"EventSummaryGenerate": EventSummaryGenerate,
	"EventDone":            EventDone,
	"EventUserInput":       EventUserInput,
	"EventDaemonLog":       EventDaemonLog,
	"EventConnected":       EventConnected,
	"EventReasoning":       EventReasoning,
	"EventTodoUpdate":      EventTodoUpdate,
	"EventCompact":         EventCompact,
	"EventUsageUpdate":     EventUsageUpdate,
	"EventUserInjected":    EventUserInjected,
	"EventCanceled":        EventCanceled,
	"EventSuggest":         EventSuggest,
	"EventPending":         EventPending,
	"EventPendingEnd":      EventPendingEnd,
	"EventClientToolCall":  EventClientToolCall,
	"EventFileChanged":     EventFileChanged,
}

func (e *EventType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if v, ok := eventTypeByName[s]; ok {
		*e = v
		return nil
	}
	return nil
}

const (
	EventText EventType = iota
	EventTextDone
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
	EventUserInput
	EventDaemonLog
	EventConnected
	EventReasoning
	EventTodoUpdate
	EventCompact
	EventUsageUpdate
	EventUserInjected
	EventCanceled
	EventTextDelta
	EventSuggest
	EventPending
	EventClientToolCall
	EventFileChanged
	EventToolProgress
	EventPendingEnd
)

func (e EventType) String() string {
	switch e {
	case EventText:
		return "EventText"
	case EventTextDelta:
		return "EventTextDelta"
	case EventTextDone:
		return "EventTextDone"
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
	case EventToolProgress:
		return "EventToolProgress"
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
	case EventFileChanged:
		return "EventFileChanged"
	case EventUserInput:
		return "EventUserInput"
	case EventDaemonLog:
		return "EventDaemonLog"
	case EventConnected:
		return "EventConnected"
	case EventReasoning:
		return "EventReasoning"
	case EventTodoUpdate:
		return "EventTodoUpdate"
	case EventCompact:
		return "EventCompact"
	case EventUsageUpdate:
		return "EventUsageUpdate"
	case EventUserInjected:
		return "EventUserInjected"
	case EventCanceled:
		return "EventCanceled"
	case EventSuggest:
		return "EventSuggest"
	case EventPending:
		return "EventPending"
	case EventPendingEnd:
		return "EventPendingEnd"
	case EventClientToolCall:
		return "EventClientToolCall"
	default:
		return "EventUnknown"
	}
}

type Event struct {
	Type            EventType                `json:"type"`
	WindowHash      string                   `json:"window_hash,omitempty"`
	TaskHash        string                   `json:"task_hash,omitempty"`
	PendingSession  string                   `json:"pending_session,omitempty"`
	Source          string                   `json:"source,omitempty"`
	Text            string                   `json:"text,omitempty"`
	ToolName        string                   `json:"tool_name,omitempty"`
	ToolArgs        string                   `json:"tool_args,omitempty"`
	ToolID          string                   `json:"tool_id,omitempty"`
	ConfirmHash     string                   `json:"confirm_hash,omitempty"`
	Result          string                   `json:"result,omitempty"`
	Model           string                   `json:"model,omitempty"`
	Reasoning       string                   `json:"reasoning,omitempty"`
	Usage           *go_llm_router.Usage     `json:"usage,omitempty"`
	UsageInput      string                   `json:"usage_input,omitempty"`
	ContextTokens   int                      `json:"context_tokens,omitempty"`
	Duration        time.Duration            `json:"duration,omitempty"`
	OutputElapsed   time.Duration            `json:"output_elapsed,omitempty"`
	Todos           []TodoItem               `json:"todos,omitempty"`
	Suggests        []string                 `json:"suggests,omitempty"`
	Files           []string                 `json:"files,omitempty"`
	Restricted      []string                 `json:"restricted,omitempty"`
	Explain         string                   `json:"explain,omitempty"`
	PasswordCached  bool                     `json:"password_cached,omitempty"`
	ClientToolCalls []go_llm_router.ToolCall `json:"client_tool_calls,omitempty"`
	Err             error                    `json:"-"`
}

func ErrorEvent(err error) Event {
	if errors.Is(err, context.Canceled) {
		return Event{Type: EventCanceled}
	}
	return Event{Type: EventError, Err: err}
}

type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form,omitempty"`
}

const (
	TodoPending    = "pending"
	TodoInProgress = "in_progress"
	TodoCompleted  = "completed"
)

func DoneEvent(model string, usage *go_llm_router.Usage, duration, outputElapsed time.Duration) Event {
	return Event{
		Type:          EventDone,
		Model:         model,
		Usage:         usage,
		Duration:      duration,
		OutputElapsed: outputElapsed,
		UsageInput:    FormatInput(InputTotals(usage)),
	}
}

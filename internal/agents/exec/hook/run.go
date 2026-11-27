package hook

import (
	"context"
	"log/slog"
	"time"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type State int

const (
	BeforeUserSend State = iota
	BeforeToolCall
	AfterToolCall
	AfterToolError
	AfterAskUser
	AfterTaskDone
	AfterTaskCancel
)

type Meta struct {
	SessionID    string
	TaskHash     string
	ToolName     string
	ToolArgs     string
	Result       string
	InputTokens  int
	OutputTokens int
	InputTotals  int
	OutputTotals int
	Hit          int
	HitTotals    int
	Input        string
	Answers      []Answer
	Duration     time.Duration
	Error        error
	Events       chan<- agenttypes.Event
}

var hookFnDic = map[State]func(context.Context, Script, Meta) (bool, string, error){
	BeforeUserSend:  beforeUserSend,
	BeforeToolCall:  beforeToolCall,
	AfterToolCall:   afterToolCall,
	AfterToolError:  afterToolError,
	AfterAskUser:    afterAskUser,
	AfterTaskDone:   afterTaskDone,
	AfterTaskCancel: afterTaskCancel,
}

func Run(ctx context.Context, scriptDic map[string]Script, state State, meta Meta) (bool, string) {
	script, ok := scriptDic[hookStateKey[state]]
	if !ok {
		return true, ""
	}

	isPass, debug, err := hookFnDic[state](ctx, script, meta)
	details := []any{
		"session", meta.SessionID,
		"task_hash", meta.TaskHash,
		"tool", meta.ToolName,
	}

	if err != nil {
		// NOTE: 不是用戶自定的錯誤，直接忽略 hook (避免後續對話全部卡死)
		slog.Debug("hook "+state.String()+": skipped", append(details, "error", err.Error())...)
		if meta.Events != nil {
			meta.Events <- agenttypes.Event{Type: agenttypes.EventHookLog, Text: state.String() + ": skipped: " + err.Error()}
		}
		return true, err.Error()
	}
	if !isPass {
		slog.Debug("[Denied] hook "+state.String()+": stop action", details...)
	} else if debug != "" {
		slog.Debug("hook "+state.String()+": passed", append(details, "debug", debug)...)
		if meta.Events != nil {
			meta.Events <- agenttypes.Event{Type: agenttypes.EventHookLog, Text: state.String() + ": " + debug}
		}
	}
	return isPass, debug
}

func Stop(ctx context.Context, scriptDic map[string]Script, state State, meta Meta) error {
	if isPass, message := Run(ctx, scriptDic, state, meta); !isPass {
		return &StopError{State: state, Message: message}
	}
	return nil
}

type StopError struct {
	State   State
	Message string
}

func (e *StopError) Error() string {
	return "[Denied] hook: " + e.Reason()
}

func (e *StopError) Reason() string {
	if e.Message == "" {
		return e.State.String()
	}
	return e.State.String() + ": " + e.Message
}

func beforeUserSend(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	return call(ctx, script, map[string]string{
		"session":   meta.SessionID,
		"task_hash": meta.TaskHash,
		"input":     meta.Input,
		"time":      time.Now().Format(time.RFC3339),
	})
}

func beforeToolCall(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	return call(ctx, script, map[string]any{
		"session":       meta.SessionID,
		"task_hash":     meta.TaskHash,
		"tool_name":     meta.ToolName,
		"tool_args":     meta.ToolArgs,
		"input_totals":  meta.InputTotals,
		"output_totals": meta.OutputTotals,
		"hit_totals":    meta.HitTotals,
		"time":          time.Now().Format(time.RFC3339),
	})
}

func afterToolCall(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	return call(ctx, script, map[string]any{
		"session":       meta.SessionID,
		"task_hash":     meta.TaskHash,
		"tool_name":     meta.ToolName,
		"tool_args":     meta.ToolArgs,
		"tool_result":   meta.Result,
		"input_tokens":  meta.InputTokens,
		"output_tokens": meta.OutputTokens,
		"hit":           meta.Hit,
		"input_totals":  meta.InputTotals,
		"output_totals": meta.OutputTotals,
		"hit_totals":    meta.HitTotals,
		"during_ms":     meta.Duration.Milliseconds(),
		"time":          time.Now().Format(time.RFC3339),
	})
}

func afterToolError(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	toolError := ""
	if meta.Error != nil {
		toolError = meta.Error.Error()
	}
	return call(ctx, script, map[string]string{
		"session":   meta.SessionID,
		"task_hash": meta.TaskHash,
		"tool_name": meta.ToolName,
		"tool_args": meta.ToolArgs,
		"error":     toolError,
		"time":      time.Now().Format(time.RFC3339),
	})
}

func afterAskUser(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	return call(ctx, script, map[string]any{
		"session":   meta.SessionID,
		"task_hash": meta.TaskHash,
		"answers":   meta.Answers,
		"time":      time.Now().Format(time.RFC3339),
	})
}

func afterTaskDone(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	return call(ctx, script, map[string]any{
		"session":       meta.SessionID,
		"task_hash":     meta.TaskHash,
		"input":         meta.Input,
		"result":        meta.Result,
		"input_totals":  meta.InputTotals,
		"output_totals": meta.OutputTotals,
		"hit_totals":    meta.HitTotals,
		"during_ms":     meta.Duration.Milliseconds(),
		"time":          time.Now().Format(time.RFC3339),
	})
}

func afterTaskCancel(ctx context.Context, script Script, meta Meta) (bool, string, error) {
	cancelError := ""
	if meta.Error != nil {
		cancelError = meta.Error.Error()
	}
	return call(ctx, script, map[string]any{
		"session":       meta.SessionID,
		"task_hash":     meta.TaskHash,
		"input":         meta.Input,
		"error":         cancelError,
		"input_totals":  meta.InputTotals,
		"output_totals": meta.OutputTotals,
		"hit_totals":    meta.HitTotals,
		"time":          time.Now().Format(time.RFC3339),
	})
}

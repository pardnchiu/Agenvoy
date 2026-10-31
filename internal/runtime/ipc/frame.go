package ipc

import (
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
)

const (
	FrameRun     = "run"
	FrameEvent   = "event"
	FrameDone    = "done"
	FrameAsk     = "ask"
	FrameReply   = "reply"
	FramePending = "pending"
	FrameCancel  = "cancel"
	FrameSteer   = "steer"
	FrameVerify  = "verify"
	FrameMCP     = "mcp"
)

type Frame struct {
	Type      string            `json:"type"`
	UUID      string            `json:"uuid,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	TaskHash  string            `json:"task_hash,omitempty"`
	Error     string            `json:"error,omitempty"`
	Canceled  bool              `json:"canceled,omitempty"`
	Pause     bool              `json:"pause,omitempty"`
	Rayload   *Payload          `json:"run,omitempty"`
	Event     *agenttypes.Event `json:"event,omitempty"`
	Ask       *Ask              `json:"ask,omitempty"`
	Reply     *Reply            `json:"reply,omitempty"`
	MCP       *MCP              `json:"mcp,omitempty"`
}

type MCP struct {
	Action  string           `json:"action,omitempty"`
	Server  string           `json:"server,omitempty"`
	Servers []mcp.ServerInfo `json:"servers,omitempty"`
	Tools   []mcp.Tool       `json:"tools,omitempty"`
}

type Ask struct {
	ID           string             `json:"id"`
	Kind         runtime.Kind       `json:"kind"`
	ToolName     string             `json:"tool_name,omitempty"`
	ToolArgs     string             `json:"tool_args,omitempty"`
	Restricted   []string           `json:"restricted,omitempty"`
	NeedPassword bool               `json:"need_password,omitempty"`
	Questions    []runtime.Question `json:"questions,omitempty"`
}

type Reply struct {
	ID        string `json:"id"`
	Approve   bool   `json:"approve,omitempty"`
	Remember  bool   `json:"remember,omitempty"`
	AllowTurn bool   `json:"allow_turn,omitempty"`
	Skip      bool   `json:"skip,omitempty"`
	Abort     bool   `json:"abort,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Answers   []any  `json:"answers,omitempty"`
	Password  string `json:"password,omitempty"`
}

type Payload struct {
	Input          string `json:"input"`
	Model          string `json:"model,omitempty"`
	Reasoning      string `json:"reasoning,omitempty"`
	WorkDir        string `json:"work_dir"`
	AllowAll       bool   `json:"allow_all,omitempty"`
	PendingTask    string `json:"pending_task,omitempty"`
	HistoryContent string `json:"history_content,omitempty"`
	WindowHash     string `json:"window_hash,omitempty"`
	Fast           bool   `json:"fast,omitempty"`
	Guide          bool   `json:"guide,omitempty"`
}

package ipc

import (
	"context"
	"errors"
	"strings"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func (c *conn) pending(ctx context.Context, f Frame) {
	sessionID := strings.TrimSpace(f.SessionID)
	if f.Rayload == nil || sessionID == "" || f.Rayload.PendingTask == "" {
		c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: "pending requires session_id and run.pending_task"})
		return
	}
	taskHash := f.Rayload.PendingTask
	info, ok := interactive.LoadPendingInfo(sessionID, taskHash)
	if !ok {
		c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: "pending task not found"})
		return
	}
	if !info.HasQuestions {
		if !interactive.ClaimOnline(sessionID, taskHash) {
			c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: "[Conflict] pending task: running"})
			return
		}
		c.resume(ctx, f, nil)
		return
	}

	questions, err := interactive.LoadPendingQuestions(sessionID, taskHash)
	if err != nil {
		c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: err.Error()})
		return
	}
	c.addWindow(f.Rayload.WindowHash)
	reply, err := runtime.Ask(agenttypes.WithWindowHash(ctx, f.Rayload.WindowHash), runtime.Request{
		Kind:      runtime.KindAskUser,
		SessionID: sessionID,
		TaskHash:  taskHash,
		Origin:    "cli-",
		ToolName:  "ask_user",
		AskUser:   &runtime.UserPayload{Questions: questions},
		Inline:    true,
	})
	if err == nil {
		err = reply.Error
	}
	if err != nil {
		if errors.Is(err, runtime.ErrUserCanceled) {
			interactive.CleanupPending(sessionID, taskHash)
		}
		c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: err.Error()})
		return
	}
	c.resume(ctx, f, reply.Answers)
}

func (c *conn) resume(ctx context.Context, f Frame, answers []any) {
	sessionID := strings.TrimSpace(f.SessionID)
	taskHash := f.Rayload.PendingTask
	allowAll := interactive.LoadPendingAllowAll(sessionID, taskHash)
	full, history, err := interactive.LoadResumeMessage(sessionID, taskHash, answers)
	if err != nil {
		if answers == nil {
			interactive.ReleaseOnline(sessionID, taskHash)
		}
		c.write(Frame{Type: FrameDone, UUID: f.UUID, SessionID: sessionID, Error: err.Error()})
		return
	}

	payload := *f.Rayload
	payload.Input = full
	payload.HistoryContent = history
	payload.AllowAll = payload.AllowAll || allowAll
	c.run(ctx, Frame{Type: FrameRun, UUID: f.UUID, SessionID: sessionID, Rayload: &payload})
}

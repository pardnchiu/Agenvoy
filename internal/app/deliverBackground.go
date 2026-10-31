package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

type backgroundResume struct {
	running bool
	dirty   bool
}

var (
	backgroundMu      sync.Mutex
	backgroundResumes = map[string]*backgroundResume{}
)

func DeliverBackground(sessionID, taskHash, workDir, id, result string) {
	if err := interactive.FinishBackground(sessionID, taskHash, id, result); err != nil {
		slog.Warn("interactive.FinishBackground",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}

	if running, _ := interactive.BackgroundState(sessionID, taskHash); running > 0 {
		status, _, _ := strings.Cut(result, "\n")
		event := agentstypes.Event{
			Type:       agentstypes.EventToolResult,
			ToolName:   interactive.BackgroundTool,
			ToolID:     id,
			TaskHash:   taskHash,
			WindowHash: interactive.LoadPendingWindow(sessionID, taskHash),
			Result:     fmt.Sprintf("%s %s; %d background task(s) still running", id, status, running),
		}
		sessionlog.Record(sessionID, event)
		pubsub.Pub(sessionID, event)
		return
	}

	backgroundMu.Lock()
	state, ok := backgroundResumes[taskHash]
	if !ok {
		state = &backgroundResume{}
		backgroundResumes[taskHash] = state
	}
	if state.running {
		state.dirty = true
		backgroundMu.Unlock()
		return
	}
	state.running = true
	backgroundMu.Unlock()

	go func() {
		for {
			resumeBackground(sessionID, taskHash, workDir)
			backgroundMu.Lock()
			if !state.dirty {
				delete(backgroundResumes, taskHash)
				backgroundMu.Unlock()
				return
			}
			state.dirty = false
			backgroundMu.Unlock()
		}
	}()
}

func resumeBackground(sessionID, taskHash, workDir string) {
	for interactive.IsOnline(sessionID, taskHash) {
		time.Sleep(500 * time.Millisecond)
	}
	if _, unreported := interactive.BackgroundState(sessionID, taskHash); unreported == 0 {
		return
	}

	input, history, err := interactive.LoadResumeMessage(sessionID, taskHash, nil)
	if err != nil {
		slog.Warn("interactive.LoadResumeMessage",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}

	origin := "chat-"
	for _, prefix := range []string{"cli-", "tg-", "dc-", "temp-"} {
		if strings.HasPrefix(sessionID, prefix) {
			origin = prefix
			break
		}
	}
	ctx := agentstypes.WithWindowHash(agentstypes.WithOrigin(context.Background(), origin), interactive.LoadPendingWindow(sessionID, taskHash))
	data := exec.Prepare(exec.ExecuteMeta{
		WorkDir:        workDir,
		Task:           input,
		Input:          input,
		SessionID:      sessionID,
		AllowAll:       interactive.LoadPendingAllowAll(sessionID, taskHash),
		PendingTask:    taskHash,
		HistoryContent: history,
		TUI:            origin == "cli-",
	})
	events, wait := exec.Stream(ctx, sessionID, 64, func(stream chan<- agentstypes.Event) error {
		return exec.Run(ctx, data, stream, false)
	})
	for range events {
	}
	if err := wait(); err != nil {
		slog.Warn("exec.Run",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
	}
}

func RecoverBackground() {
	for _, ref := range interactive.UnfinishedBackground() {
		go DeliverBackground(ref.SessionID, ref.TaskHash, ref.WorkDir, ref.ID, "failed: the daemon restarted before this background leg finished")
	}
}

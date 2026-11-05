package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
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
		if errors.Is(err, interactive.ErrBackgroundCanceled) {
			return
		}
		slog.Warn("interactive.FinishBackground",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}

	ScheduleBackgroundResume(sessionID, taskHash, workDir)
}

func DeliverBackgroundUpdate(sessionID, taskHash, workDir, id, text string) {
	if err := interactive.UpdateBackground(sessionID, taskHash, id, text); err != nil {
		slog.Warn("interactive.UpdateBackground",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}
	ScheduleBackgroundResume(sessionID, taskHash, workDir)
}

func ScheduleBackgroundResume(sessionID, taskHash, workDir string) {
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
	if _, unreported := interactive.BackgroundState(sessionID, taskHash); unreported == 0 {
		return
	}
	for {
		if questions, _ := interactive.LoadPendingQuestions(sessionID, taskHash); len(questions) > 0 {
			return
		}
		if interactive.ClaimOnline(sessionID, taskHash) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if _, unreported := interactive.BackgroundState(sessionID, taskHash); unreported == 0 {
		interactive.ReleaseOnline(sessionID, taskHash)
		return
	}

	input, history, err := interactive.LoadResumeMessage(sessionID, taskHash, nil)
	if err != nil {
		interactive.ReleaseOnline(sessionID, taskHash)
		slog.Warn("interactive.LoadResumeMessage",
			slog.String("session", sessionID),
			slog.String("task", taskHash),
			slog.String("error", err.Error()))
		return
	}

	origin := runtime.OriginOf(sessionID)
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

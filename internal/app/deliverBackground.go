package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

func DeliverBackground(sessionID, jobID, id, result string) {
	parentTask, err := interactive.FinishBackground(sessionID, jobID, id, result)
	if err != nil {
		if errors.Is(err, interactive.ErrBackgroundCanceled) {
			return
		}
		slog.Warn("interactive.FinishBackground",
			slog.String("session", sessionID),
			slog.String("job", jobID),
			slog.String("error", err.Error()))
		return
	}
	interactive.SettleBackground(sessionID, parentTask)
}

func DeliverBackgroundUpdate(sessionID, jobID, id, text string) {
	if _, err := interactive.UpdateBackground(sessionID, jobID, id, text); err != nil {
		slog.Warn("interactive.UpdateBackground",
			slog.String("session", sessionID),
			slog.String("job", jobID),
			slog.String("error", err.Error()))
	}
}

func RunBackgroundJob(job interactive.BackgroundJob) {
	go func() {
		input := interactive.BackgroundJobInput(job)
		origin := runtime.OriginOf(job.SessionID)
		ctx := agentstypes.WithWindowHash(agentstypes.WithOrigin(context.Background(), origin), job.WindowHash)
		data := exec.Prepare(exec.ExecuteMeta{
			WorkDir:   job.WorkDir,
			Task:      input,
			Input:     input,
			SessionID: job.SessionID,
			AllowAll:  job.AllowAll,
			TUI:       origin == "cli-",
		})
		events, wait := exec.Stream(ctx, job.SessionID, 64, func(stream chan<- agentstypes.Event) error {
			return exec.Run(ctx, data, stream, false)
		})
		for range events {
		}
		if err := wait(); err != nil {
			slog.Warn("exec.Run",
				slog.String("session", job.SessionID),
				slog.String("job", job.ID),
				slog.String("error", err.Error()))
		}
	}()
}

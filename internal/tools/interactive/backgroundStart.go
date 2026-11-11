package interactive

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type BackgroundDeliver func(sessionID, jobID, id, result string)

var (
	backgroundDeliver BackgroundDeliver
	backgroundUpdate  BackgroundDeliver
	backgroundTrigger func(job BackgroundJob)
)

func SetBackgroundDeliver(deliver, update BackgroundDeliver, trigger func(job BackgroundJob)) {
	backgroundDeliver = deliver
	backgroundUpdate = update
	backgroundTrigger = trigger
}

func BackgroundGoalProperties() map[string]any {
	return map[string]any{
		"objective": map[string]any{
			"type":        "string",
			"description": "What this background group is for, written to stand alone: once this task has ended, the follow-up run sees only objective, next, the leg reports and this task's id. Legs started in the same turn form one group; the first non-empty value wins.",
		},
		"next": map[string]any{
			"type":        "string",
			"description": "What to do with the reports once every leg of the group has finished after this task has ended, e.g. 'compare the three reports and push a summary to telegram'. While this task is still open, each report arrives here instead.",
		},
	}
}

func BackgroundAllowed(ctx context.Context) bool {
	return agenttypes.OriginFrom(ctx) == "cli-" && agenttypes.DeliverToFrom(ctx) == ""
}

func StartBackground(ctx context.Context, sessionID, taskHash, group, workDir string, goal BackgroundGoal, prefix, description string, run func(ctx context.Context, report func(string), log io.Writer) string) (string, error) {
	if taskHash == "" || backgroundDeliver == nil {
		return "", utilsmsg.New(utilsmsg.Unavailable, "background", "this request has no session task to resume; run it without background")
	}
	if err := AcquireBackgroundSlot(sessionID); err != nil {
		return "", err
	}

	id := prefix + "_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8]
	logDir := filesystem.BackgroundDir(sessionID)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		ReleaseBackgroundSlot(sessionID)
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.MkdirAll", logDir, err)
	}
	logPath := filesystem.BackgroundLogPath(sessionID, id)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0o600)
	if err != nil {
		ReleaseBackgroundSlot(sessionID)
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.OpenFile", logPath, err)
	}
	removeLog := func() {
		logFile.Close()
		os.Remove(logPath)
	}
	jobID, err := addLeg(sessionID, taskHash, group, workDir, agenttypes.WindowHash(ctx), goal, id, description)
	if err != nil {
		removeLog()
		ReleaseBackgroundSlot(sessionID)
		return "", err
	}

	bgCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	RegisterBackgroundCancel(sessionID, taskHash, id, cancel)
	go func() {
		defer ReleaseBackgroundSlot(sessionID)
		defer cancel()
		defer removeLog()
		result := "failed: background task ended without a result"
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[Internal] background task: panic recovered",
					slog.String("session", sessionID),
					slog.String("id", id),
					slog.Any("panic", r))
				result = fmt.Sprintf("failed: panic: %v", r)
			}
			backgroundDeliver(sessionID, jobID, id, result)
		}()
		result = run(bgCtx, func(text string) {
			if backgroundUpdate != nil && bgCtx.Err() == nil {
				backgroundUpdate(sessionID, jobID, id, text)
			}
		}, logFile)
	}()
	return id, nil
}

package interactive

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type BackgroundDeliver func(sessionID, taskHash, workDir, id, result string)

var (
	backgroundDeliver BackgroundDeliver
	backgroundUpdate  BackgroundDeliver
	backgroundResume  func(sessionID, taskHash, workDir string)
)

func SetBackgroundDeliver(deliver, update BackgroundDeliver, resume func(sessionID, taskHash, workDir string)) {
	backgroundDeliver = deliver
	backgroundUpdate = update
	backgroundResume = resume
}

func BackgroundAllowed(ctx context.Context) bool {
	return agenttypes.OriginFrom(ctx) == "cli-" && agenttypes.DeliverToFrom(ctx) == ""
}

func StartBackground(ctx context.Context, sessionID, taskHash, workDir, prefix, description string, run func(ctx context.Context, report func(string)) string) (string, error) {
	if taskHash == "" || backgroundDeliver == nil {
		return "", utilsmsg.New(utilsmsg.Unavailable, "background", "this request has no session task to resume; run it without background")
	}
	if err := AcquireBackgroundSlot(sessionID); err != nil {
		return "", err
	}

	id := prefix + "_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8]
	if err := AddBackground(sessionID, taskHash, workDir, id, description); err != nil {
		ReleaseBackgroundSlot(sessionID)
		return "", err
	}

	bgCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	RegisterBackgroundCancel(sessionID, taskHash, id, cancel)
	go func() {
		defer ReleaseBackgroundSlot(sessionID)
		defer cancel()
		result := "failed: background task ended without a result"
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[Internal] background task: panic recovered",
					slog.String("session", sessionID),
					slog.String("id", id),
					slog.Any("panic", r))
				result = fmt.Sprintf("failed: panic: %v", r)
			}
			backgroundDeliver(sessionID, taskHash, workDir, id, result)
		}()
		result = run(bgCtx, func(text string) {
			if backgroundUpdate != nil && bgCtx.Err() == nil {
				backgroundUpdate(sessionID, taskHash, workDir, id, text)
			}
		})
	}()
	return id, nil
}

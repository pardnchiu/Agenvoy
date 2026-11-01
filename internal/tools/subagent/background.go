package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	execsubagent "github.com/pardnchiu/agenvoy/internal/agents/exec/subagent"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Deliver func(sessionID, taskHash, workDir, id, result string)

var deliver Deliver

func SetDeliver(fn Deliver) {
	deliver = fn
}

func startBackground(ctx context.Context, e *tooltypes.Executor, params invokeParams, task, sessionID, model, reasoning string, excludeTools []string, ignoreHistory bool) (string, error) {
	if e.PendingTask == "" || deliver == nil {
		return "", utilsmsg.New(utilsmsg.Unavailable, "background", "this request has no session task to resume; run the leg without background")
	}
	if err := interactive.AcquireBackgroundSlot(e.SessionID); err != nil {
		return "", err
	}

	id := "sub_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8]
	description := strings.TrimSpace(params.ReportName)
	if description == "" {
		description = go_pkg_utils.TruncateString(task, 48)
	}
	parentSession, parentTask, workDir := e.SessionID, e.PendingTask, e.WorkDir
	if err := interactive.AddBackground(parentSession, parentTask, workDir, id, description); err != nil {
		interactive.ReleaseBackgroundSlot(parentSession)
		return "", err
	}

	bgCtx := execsubagent.WithBackground(context.WithoutCancel(ctx), params.TimeoutMin)
	systemPrompt := strings.TrimSpace(params.SystemPrompt)
	go func() {
		defer interactive.ReleaseBackgroundSlot(parentSession)
		result := "failed: background leg ended without a result"
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[Internal] background subagent: panic recovered",
					slog.String("session", parentSession),
					slog.Any("panic", r))
				result = fmt.Sprintf("failed: panic: %v", r)
			}
			deliver(parentSession, parentTask, workDir, id, result)
		}()

		output, err := execsubagent.Exec(bgCtx, task, sessionID, model, reasoning, systemPrompt, excludeTools, ignoreHistory, false)
		if err != nil {
			result = "failed: " + err.Error()
			return
		}
		saved, err := saveReport(output, params.ReportName)
		if err != nil {
			result = "failed: " + err.Error()
			return
		}
		result = saved
	}()

	return fmt.Sprintf("background leg %s started. This task pauses after this step and resumes with every background leg's report once all of them have finished.", id), nil
}

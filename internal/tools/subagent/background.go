package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	execsubagent "github.com/pardnchiu/agenvoy/internal/agents/exec/subagent"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Deliver func(sessionID, taskHash, workDir, id, result string)

var (
	deliver         Deliver
	backgroundSlots = make(chan struct{}, configs.MAX_BACKGROUND_SUBAGENTS)
)

func SetDeliver(fn Deliver) {
	deliver = fn
}

func startBackground(ctx context.Context, e *tooltypes.Executor, params invokeParams, task, sessionID, model, reasoning string, excludeTools []string, ignoreHistory bool) (string, error) {
	if e.PendingTask == "" || deliver == nil {
		return "", utilsmsg.New(utilsmsg.Unavailable, "background", "this request has no session task to resume; run the leg without background")
	}
	select {
	case backgroundSlots <- struct{}{}:
	default:
		return "", utilsmsg.New(utilsmsg.Limit, "background", "%d background legs are already running; run this leg without background or wait for one to finish", configs.MAX_BACKGROUND_SUBAGENTS)
	}

	id := "sub_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8]
	description := strings.TrimSpace(params.ReportName)
	if description == "" {
		description = go_pkg_utils.TruncateString(task, 48)
	}
	parentSession, parentTask, workDir := e.SessionID, e.PendingTask, e.WorkDir
	if err := interactive.AddBackground(parentSession, parentTask, workDir, id, description); err != nil {
		<-backgroundSlots
		return "", err
	}

	bgCtx := execsubagent.WithBackground(context.WithoutCancel(ctx))
	systemPrompt := strings.TrimSpace(params.SystemPrompt)
	go func() {
		defer func() { <-backgroundSlots }()
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

	return fmt.Sprintf("[subagents] background leg %s started. This task pauses after this step and resumes with every background leg's report once all of them have finished.", id), nil
}

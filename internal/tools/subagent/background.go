package subagent

import (
	"context"
	"fmt"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	execsubagent "github.com/pardnchiu/agenvoy/internal/agents/exec/subagent"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func startBackground(ctx context.Context, e *tooltypes.Executor, params invokeParams, task, sessionID, model, reasoning string, excludeTools []string, ignoreHistory bool) (string, error) {
	description := strings.TrimSpace(params.ReportName)
	if description == "" {
		description = go_pkg_utils.TruncateString(task, 48)
	}
	systemPrompt := strings.TrimSpace(params.SystemPrompt)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "sub", description, func(ctx context.Context, _ func(string)) string {
		output, err := execsubagent.Exec(execsubagent.WithBackground(ctx, params.TimeoutMin), task, sessionID, model, reasoning, systemPrompt, excludeTools, ignoreHistory, false)
		if err != nil {
			return "failed: " + err.Error()
		}
		saved, err := saveReport(output, params.ReportName)
		if err != nil {
			return "failed: " + err.Error()
		}
		return saved
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("background leg %s started. Keep working on this turn; if it ends first, the task stays open and resumes with this leg's report as soon as it finishes, without waiting for other background tasks.", id), nil
}

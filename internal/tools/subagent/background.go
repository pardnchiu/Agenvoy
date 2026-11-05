package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	execsubagent "github.com/pardnchiu/agenvoy/internal/agents/exec/subagent"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registBackgroundSubagents() {
	properties := invokeProperties()
	properties["timeout_min"] = map[string]any{
		"type":        "integer",
		"description": fmt.Sprintf("Minutes before the leg is stopped and reported as failed, 1-%d. Set it near the expected research time so a stuck leg fails early instead of holding the task for the full window.", configs.MAX_BACKGROUND_TIMEOUT_MIN),
		"minimum":     1,
		"maximum":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
		"default":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
	}

	toolregister.Regist(toolregister.Def{
		Name:        "background_subagents",
		SystemUse:   false,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: fmt.Sprintf(`Starts a subtask in its own session in the background and returns a task id at once; the leg's saved report resumes this task when it finishes.
Use when every leg is very likely to take over %d minutes end to end (multi-source research, broad data gathering), or the user says 慢慢查 / 跑完告訴我 / 背景跑. Use it only for the gathering that opens a request, dispatching every leg in one go: each report resumes this task with only the objective and that report, none of this turn's tool results. At most %d background tasks per session, background_command included.
A leg of a few calls, one whose result decides your next tool call, or a named-agent lookup → subagents.`, configs.BACKGROUND_THRESHOLD_MIN, configs.MAX_BACKGROUND_TASKS),
		Parameters: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []string{"task", "model"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params invokeParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return invokeSubagent(ctx, e, params, true)
		},
	})
}

func startBackground(ctx context.Context, e *tooltypes.Executor, params invokeParams, task, sessionID, model, reasoning string, excludeTools []string, ignoreHistory bool) (string, error) {
	description := strings.TrimSpace(params.ReportName)
	if description == "" {
		description = go_pkg_utils.TruncateString(task, 48)
	}
	systemPrompt := strings.TrimSpace(params.SystemPrompt)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "sub", description, func(ctx context.Context, _ func(string), log io.Writer) string {
		output, err := execsubagent.Exec(execsubagent.WithBackground(ctx, params.TimeoutMin, log), task, sessionID, model, reasoning, systemPrompt, excludeTools, ignoreHistory, false)
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

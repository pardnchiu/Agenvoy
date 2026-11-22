package exec

import (
	"context"
	"log/slog"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/readonly"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const decisionCallSafe = "safe"

var safetyCriteria = map[string]any{
	decisionCallSafe: "Low impact even if unwanted: creating a new file or directory (`targets` shows `exists: false`), writing or editing ordinary files of the task, reading, previewing or dry-running, installing packages, builds and tests, or removing files the task itself created. Being outside `work_dir` alone is not a risk.",
	"dangerous":      "Real loss or exposure: overwriting or deleting a file that already existed (`exists: true`) and holds user data, discarding uncommitted work or rewriting git history, changing credentials, keys, shell profiles or app config, changing remote state (push, deploy, publish, cloud, database or API writes), escalating privileges, or sending secrets or private files out.",
}

func isMutatingCommand(toolName, toolArgs string) bool {
	return (toolName == "run_command" || toolName == "background_command") && !readonly.IsRunCommand(toolArgs)
}

func unsafeByDecision(ctx context.Context, sessionID string, exec *tooltypes.Executor, toolName, toolArgs string, paths []string, flagged bool) (unsafe, decided bool) {
	agent, name, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return false, false
	}
	request, _ := decision.Request(exec.Objective)
	targets := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		targets = append(targets, map[string]any{"path": path, "exists": go_pkg_filesystem_reader.Exists(path)})
	}
	answers, err := decision.Decide(ctx, agent, map[string]any{
		"request":      request,
		"work_dir":     exec.WorkDir,
		"tool":         toolName,
		"targets":      targets,
		"args":         go_pkg_utils.TruncateString(toolArgs, configs.MAX_DECISION_RUNES),
		"rule_flagged": flagged,
	}, map[string]go_llm_router.DecisionQuestion{
		"safety": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Can this tool call run without asking the user first?",
				"focus":    "Judge the effect the args actually produce, not the tool name or where the path sits. This call reached you because it touches a path outside `work_dir`, a sensitive path, or matched a static rule; most such calls are routine. `rule_flagged` true means a pattern rule (reset --hard, clean, force push, deploy, migration, destructive SQL, piping a download into an interpreter) matched and may be a false positive: dangerous only when that effect really happens.",
			},
			Criteria: safetyCriteria,
		},
	})
	if err != nil {
		slog.Debug("[Unavailable] tool safety check: keep default rules",
			slog.String("session", sessionID),
			slog.String("model", name),
			slog.String("tool", toolName),
			slog.String("error", err.Error()))
		return false, false
	}
	unsafe = !decision.Confident(answers["safety"], decisionCallSafe)
	slog.Debug("tool safety check",
		slog.String("session", sessionID),
		slog.String("tool", toolName),
		slog.Bool("flagged", flagged),
		slog.Bool("unsafe", unsafe))
	return unsafe, true
}

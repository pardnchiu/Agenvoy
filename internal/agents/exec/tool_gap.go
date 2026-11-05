package exec

import (
	"context"
	"log/slog"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const decisionToolMissing = "missing"

var toolGapCriteria = map[string]any{
	"covered":           "An existing tool in `tools` handles it, or it needs no tool at all.",
	decisionToolMissing: "It needs a dedicated capability that no tool in `tools` provides.",
}

func detectToolGap(ctx context.Context, sessionID string, exec *tooltypes.Executor) bool {
	request, ok := decision.Request(exec.Objective)
	if !ok || exec.ExcludeTools["reasoning_guide"] || exec.ExcludeTools["edit_tool"] {
		return false
	}
	agent, name, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return false
	}

	names := make([]string, 0, len(exec.AllTools))
	for _, tool := range exec.AllTools {
		if !exec.ExcludeTools[tool.Function.Name] {
			names = append(names, tool.Function.Name)
		}
	}
	answers, err := decision.Decide(ctx, agent, map[string]any{
		"request": request,
		"tools":   names,
	}, map[string]go_llm_router.DecisionQuestion{
		"tool": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Does fulfilling `request` need a capability that no tool in `tools` provides?",
				"focus":    "Judge by tool names. A recurring live-data capability (weather, FX rate, stock or crypto quote, geocoding, air quality, translation API...) is covered only by a dedicated api_*/script_*/ext_* tool; general tools such as http_request, run_command, fetch_page or search_web do not count as covering it. Requests answered by reasoning, conversation, file or code work, or a web search need no new tool.",
			},
			Criteria: toolGapCriteria,
		},
	})
	if err != nil {
		slog.Debug("[Unavailable] tool gap check: skip",
			slog.String("session", sessionID),
			slog.String("model", name),
			slog.String("error", err.Error()))
		return false
	}
	missing := decision.Confident(answers["tool"], decisionToolMissing)
	slog.Debug("tool gap check",
		slog.String("session", sessionID),
		slog.Int("tools", len(names)),
		slog.Bool("missing", missing))
	return missing
}

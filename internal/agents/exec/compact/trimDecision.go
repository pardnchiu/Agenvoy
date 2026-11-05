package compact

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
)

const decisionStepUnneeded = "unneeded"

var stepCriteria = map[string]any{
	"needed":             "The step's result is still used to answer `request`, or a later step builds on it.",
	decisionStepUnneeded: "Dropping the step loses nothing toward `request`: a failed or empty call, a result superseded by a later step, or data about something `request` does not ask for.",
}

type toolStep struct {
	call  string
	reply string
}

func pruneToolsWithDecision(ctx context.Context, sessionID, objective string, toolCall *[]go_llm_router.Message) {
	request, ok := decision.Request(objective)
	if !ok {
		return
	}
	steps, owner := groupToolSteps(*toolCall)
	if len(steps) < 2 {
		return
	}
	candidates := steps[:len(steps)-1]

	agent, name, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return
	}

	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
	drop := make([]bool, len(steps))
	var wg sync.WaitGroup
	for _, batch := range decision.Batch(len(candidates), budget, func(i int) int {
		return utf8.RuneCountInString(candidates[i].call) + utf8.RuneCountInString(candidates[i].reply)
	}) {
		wg.Go(func() {
			state := make(map[string]any, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				key := fmt.Sprintf("g%d", i)
				state[key] = map[string]string{"call": candidates[i].call, "result": candidates[i].reply}
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": "Is tool step `" + key + "` in `steps` still needed to answer `request`?",
						"focus":    "Steps are in call order; a later step can supersede an earlier one. When unsure, needed.",
					},
					Criteria: stepCriteria,
				}
			}
			answers, err := decision.Decide(ctx, agent, map[string]any{"request": request, "steps": state}, questions)
			if err != nil {
				slog.Debug("[Unavailable] trim tool step check: keep batch",
					slog.String("session", sessionID),
					slog.String("model", name),
					slog.String("error", err.Error()))
				return
			}
			for _, i := range batch {
				drop[i] = decision.Confident(answers[fmt.Sprintf("g%d", i)], decisionStepUnneeded)
			}
		})
	}
	wg.Wait()

	dropped := 0
	for _, one := range drop {
		if one {
			dropped++
		}
	}
	if dropped == 0 {
		return
	}

	kept := make([]go_llm_router.Message, 0, len(*toolCall))
	for i, message := range *toolCall {
		if step := owner[i]; step < 0 || !drop[step] {
			kept = append(kept, message)
		}
	}
	slog.Debug("trim tool step filter",
		slog.String("session", sessionID),
		slog.Int("steps", len(steps)),
		slog.Int("unneeded", dropped))
	*toolCall = kept
}

func groupToolSteps(list []go_llm_router.Message) ([]toolStep, []int) {
	var steps []toolStep
	owner := make([]int, len(list))
	idStep := map[string]int{}
	for i, message := range list {
		owner[i] = -1
		switch {
		case message.Role == "assistant" && len(message.ToolCalls) > 0:
			if slices.ContainsFunc(message.ToolCalls, func(tool go_llm_router.ToolCall) bool { return tool.Function.Name == "write_todo" }) {
				continue
			}
			var step toolStep
			var calls []string
			for _, tool := range message.ToolCalls {
				idStep[tool.ID] = len(steps)
				calls = append(calls, tool.Function.Name+"("+tool.Function.Arguments+")")
			}
			step.call = go_pkg_utils.TruncateString(strings.Join(calls, "\n"), configs.MAX_DECISION_RUNES)
			owner[i] = len(steps)
			steps = append(steps, step)

		case message.Role == "tool" && message.ToolCallID != "":
			index, ok := idStep[message.ToolCallID]
			if !ok {
				continue
			}
			owner[i] = index
			step := &steps[index]
			step.reply = go_pkg_utils.TruncateString(strings.TrimSpace(step.reply+"\n"+extractMessage(message)), configs.MAX_DECISION_RUNES)
		}
	}
	return steps, owner
}

package agentSummary

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	sessionhistory "github.com/pardnchiu/agenvoy/internal/session/history"
)

const (
	decisionNoise    = "noise"
	decisionOutdated = "outdated"
)

var (
	noiseCriteria = map[string]any{
		"keep":        "The exchange carries a request, result, decision, preference or fact worth remembering.",
		decisionNoise: "Removing the exchange loses nothing the summary should record.",
	}
	decisionCriteria = map[string]any{
		"current":        "Still holds, or `exchanges` say nothing that contradicts it.",
		decisionOutdated: "`exchanges` reverse, supersede or explicitly drop it.",
	}
	discussionCriteria = map[string]any{
		"current":        "Still a useful record of a topic.",
		decisionOutdated: "Superseded by `exchanges`, or trivial and no longer load-bearing.",
	}
)

type summaryExchange struct {
	records []sessionhistory.Record
	user    string
	reply   string
}

func filterWithDecision(ctx context.Context, sessionID string, dic map[string]any, histories []sessionhistory.Record) ([]sessionhistory.Record, map[string]any, bool) {
	agent, name, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return histories, dic, false
	}

	raw, _ := json.Marshal(dic)
	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCount(raw)

	list := groupSummaryExchanges(histories)
	kept := dropNoise(ctx, agent, sessionID, name, dic, list, budget)
	pruned, stale := dropStale(ctx, agent, sessionID, name, dic, kept, budget)

	slog.Debug("summary decision filter",
		slog.String("session", sessionID),
		slog.Int("exchanges", len(list)),
		slog.Int("noise", len(list)-len(kept)),
		slog.Int("stale", stale))

	out := make([]sessionhistory.Record, 0, len(histories))
	for _, one := range kept {
		out = append(out, one.records...)
	}
	return out, pruned, true
}

func groupSummaryExchanges(histories []sessionhistory.Record) []summaryExchange {
	var list []summaryExchange
	for _, record := range histories {
		if record.Role == "user" || len(list) == 0 {
			list = append(list, summaryExchange{})
		}
		last := &list[len(list)-1]
		last.records = append(last.records, record)
		text := strings.TrimSpace(record.Text())
		if text == "" {
			continue
		}
		if record.Role == "user" {
			last.user = go_pkg_utils.TruncateString(text, configs.MAX_DECISION_RUNES)
			continue
		}
		last.reply = go_pkg_utils.TruncateString(strings.TrimSpace(last.reply+" "+text), configs.MAX_DECISION_RUNES)
	}
	return list
}

func batchExchanges(list []summaryExchange, budget int) [][]int {
	return decision.Batch(len(list), budget, func(i int) int {
		return utf8.RuneCountInString(list[i].user) + utf8.RuneCountInString(list[i].reply)
	})
}

func exchangeState(list []summaryExchange, batch []int) map[string]any {
	dic := make(map[string]any, len(batch))
	for _, i := range batch {
		dic[fmt.Sprintf("e%d", i)] = map[string]string{"user": list[i].user, "reply": list[i].reply}
	}
	return dic
}

func dropNoise(ctx context.Context, agent go_llm_router.DecisionAgent, sessionID, name string, dic map[string]any, list []summaryExchange, budget int) []summaryExchange {
	noise := make([]bool, len(list))
	for _, batch := range batchExchanges(list, budget) {
		questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
		for _, i := range batch {
			key := fmt.Sprintf("e%d", i)
			questions[key] = go_llm_router.DecisionQuestion{
				Type: go_llm_router.DecisionChoice,
				Instructions: map[string]any{
					"question": "Is exchange `" + key + "` in `exchanges` noise for the long-term `summary`?",
					"focus":    "Noise = greetings, thanks, acknowledgements, retries of the same request, failed or canceled turns with no outcome, or small talk that changes nothing `summary` records.",
				},
				Criteria: noiseCriteria,
			}
		}
		answers, err := decision.Decide(ctx, agent, map[string]any{"summary": dic, "exchanges": exchangeState(list, batch)}, questions)
		if err != nil {
			slog.Debug("[Unavailable] summary noise check: keep batch",
				slog.String("session", sessionID),
				slog.String("model", name),
				slog.String("error", err.Error()))
			continue
		}
		for _, i := range batch {
			noise[i] = decision.Confident(answers[fmt.Sprintf("e%d", i)], decisionNoise)
		}
	}

	kept := make([]summaryExchange, 0, len(list))
	for i, one := range list {
		if !noise[i] {
			kept = append(kept, one)
		}
	}
	return kept
}

func dropStale(ctx context.Context, agent go_llm_router.DecisionAgent, sessionID, name string, dic map[string]any, list []summaryExchange, budget int) (map[string]any, int) {
	decisions, _ := dic["key_decisions"].([]any)
	discussions, _ := dic["past_discussions"].([]any)
	if len(list) == 0 || len(decisions)+len(discussions) == 0 {
		return dic, 0
	}

	questions := make(map[string]go_llm_router.DecisionQuestion, len(decisions)+len(discussions))
	for i := range decisions {
		questions[fmt.Sprintf("k%d", i)] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": fmt.Sprintf("Is `summary.key_decisions[%d]` outdated given `exchanges`?", i),
				"focus":    "Outdated only when `exchanges` reverse, replace or explicitly drop it; being unrelated to `exchanges` is still current.",
			},
			Criteria: decisionCriteria,
		}
	}
	for i := range discussions {
		questions[fmt.Sprintf("p%d", i)] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": fmt.Sprintf("Is `summary.past_discussions[%d]` outdated given `exchanges`?", i),
				"focus":    "Outdated when `exchanges` supersede its direction or it is a trivial topic (greetings) that no longer matters; age alone is not a reason.",
			},
			Criteria: discussionCriteria,
		}
	}

	stale := map[string]bool{}
	for _, batch := range batchExchanges(list, budget) {
		answers, err := decision.Decide(ctx, agent, map[string]any{"summary": dic, "exchanges": exchangeState(list, batch)}, questions)
		if err != nil {
			slog.Debug("[Unavailable] summary stale check: keep entries",
				slog.String("session", sessionID),
				slog.String("model", name),
				slog.String("error", err.Error()))
			continue
		}
		for key := range questions {
			if decision.Confident(answers[key], decisionOutdated) {
				stale[key] = true
			}
		}
	}
	if len(stale) == 0 {
		return dic, 0
	}

	out := maps.Clone(dic)
	out["key_decisions"] = keepFresh(decisions, "k", stale)
	out["past_discussions"] = keepFresh(discussions, "p", stale)
	return out, len(stale)
}

func keepFresh(items []any, prefix string, stale map[string]bool) []any {
	list := make([]any, 0, len(items))
	for i, item := range items {
		if !stale[fmt.Sprintf("%s%d", prefix, i)] {
			list = append(list, item)
		}
	}
	return list
}

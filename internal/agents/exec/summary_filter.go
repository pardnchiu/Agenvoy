package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	sessionsummary "github.com/pardnchiu/agenvoy/internal/session/summary"
)

const decisionSummaryUnrelated = "unrelated"

var summaryCriteria = map[string]any{
	"relevant":               "`request` continues, builds on or is constrained by the entry, `request` asks what was discussed, decided or remembered, or the entry is a general rule that applies regardless of topic (language, format, workflow).",
	decisionSummaryUnrelated: "The entry is about a different topic than `request`.",
}

type summaryEntry struct {
	field string
	index int
	value any
}

func relevantSummary(ctx context.Context, sessionID, objective string, dic map[string]any) string {
	if len(dic) == 0 {
		return ""
	}

	var entries []summaryEntry
	for _, field := range []string{"key_decisions", "past_discussions"} {
		items, _ := dic[field].([]any)
		for i, item := range items {
			entries = append(entries, summaryEntry{field: field, index: i, value: item})
		}
	}
	if current, ok := dic["current_discussion"]; ok && current != nil {
		entries = append(entries, summaryEntry{field: "current_discussion", index: -1, value: current})
	}
	if len(entries) == 0 {
		return ""
	}

	request, ok := decision.Request(objective)
	if !ok {
		return sessionsummary.Prompt(dic)
	}
	agent, model, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return sessionsummary.Prompt(dic)
	}

	sizes := make([]int, len(entries))
	for i, e := range entries {
		raw, _ := json.Marshal(e.value)
		sizes[i] = utf8.RuneCount(raw)
	}

	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
	drop := make([]bool, len(entries))
	var wg sync.WaitGroup
	for _, batch := range decision.Batch(len(entries), budget, func(i int) int { return sizes[i] }) {
		wg.Go(func() {
			state := make(map[string]any, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				key := fmt.Sprintf("e%d", i)
				state[key] = entries[i].value
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": "Is summary entry `" + key + "` in `entries` relevant to `request`?",
						"focus":    "Judge by the entry's topic and content, not by its age.",
					},
					Criteria: summaryCriteria,
				}
			}
			answers, err := decision.Decide(ctx, agent, map[string]any{"request": request, "entries": state}, questions)
			if err != nil {
				slog.Debug("[Unavailable] summary relevance check: keep batch",
					slog.String("session", sessionID),
					slog.String("model", model),
					slog.String("error", err.Error()))
				return
			}
			for _, i := range batch {
				drop[i] = decision.Confident(answers[fmt.Sprintf("e%d", i)], decisionSummaryUnrelated)
			}
		})
	}
	wg.Wait()

	out := maps.Clone(dic)
	lists := map[string][]any{}
	kept := 0
	for i, e := range entries {
		if drop[i] {
			continue
		}
		kept++
		if e.index < 0 {
			continue
		}
		lists[e.field] = append(lists[e.field], e.value)
	}
	for _, field := range []string{"key_decisions", "past_discussions"} {
		if _, ok := dic[field]; ok {
			out[field] = append([]any{}, lists[field]...)
		}
	}
	for i, e := range entries {
		if e.index < 0 && drop[i] {
			delete(out, e.field)
		}
	}
	slog.Debug("summary relevance check",
		slog.String("session", sessionID),
		slog.Int("entries", len(entries)),
		slog.Int("kept", kept))
	if kept == 0 {
		return ""
	}
	return sessionsummary.Prompt(out)
}

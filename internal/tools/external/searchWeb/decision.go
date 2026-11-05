package searchWeb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const decisionUnrelated = "unrelated"

var relatedCriteria = map[string]any{
	"related":         "The result's title bears on `request`, even partly or as background.",
	decisionUnrelated: "The result's title is about a different subject than `request`.",
}

func keepRelated(ctx context.Context, objective, query, raw string) string {
	request, ok := decision.Request(objective)
	if !ok {
		return raw
	}
	agent, name, ok := decision.Ready(ctx, slog.String("query", query))
	if !ok {
		return raw
	}
	var out searchResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return raw
	}
	var web, news []json.RawMessage
	_ = json.Unmarshal(out.Web, &web)
	_ = json.Unmarshal(out.News, &news)
	total := len(web) + len(news)
	if total == 0 {
		return raw
	}

	titles := make(map[string]string, total)
	questions := make(map[string]go_llm_router.DecisionQuestion, total)
	for i, item := range append(append([]json.RawMessage{}, web...), news...) {
		var one struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(item, &one) != nil || strings.TrimSpace(one.Title) == "" {
			continue
		}
		key := fmt.Sprintf("t%d", i)
		titles[key] = one.Title
		questions[key] = go_llm_router.DecisionQuestion{
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Does result `" + key + "` in `titles` bear on `request`?",
				"focus":    "`query` is the search issued for `request`; judge the title against `request`. When unsure, related.",
			},
			Criteria: relatedCriteria,
		}
	}
	if len(questions) == 0 {
		return raw
	}

	answers, err := decision.Decide(ctx, agent, map[string]any{
		"request": request,
		"query":   query,
		"titles":  titles,
	}, questions)
	if err != nil {
		slog.Debug("[Unavailable] search_web relevance check: keep results",
			slog.String("query", query),
			slog.String("model", name),
			slog.String("error", err.Error()))
		return raw
	}

	keep := func(list []json.RawMessage, offset int) []json.RawMessage {
		kept := make([]json.RawMessage, 0, len(list))
		for i, item := range list {
			if !decision.Confident(answers[fmt.Sprintf("t%d", offset+i)], decisionUnrelated) {
				kept = append(kept, item)
			}
		}
		return kept
	}
	keptWeb, keptNews := keep(web, 0), keep(news, len(web))
	dropped := total - len(keptWeb) - len(keptNews)
	slog.Debug("search_web relevance filter",
		slog.String("query", query),
		slog.Int("results", total),
		slog.Int("unrelated", dropped))
	if dropped == 0 {
		return raw
	}
	if len(keptWeb)+len(keptNews) == 0 {
		return fmt.Sprintf("No search result bears on the current request: all %d results for %q were about other subjects. Refine the query.", total, query)
	}

	if out.Web != nil {
		out.Web, _ = json.Marshal(keptWeb)
	}
	if out.News != nil {
		out.News, _ = json.Marshal(keptNews)
	}
	filtered, err := utils.MarshalPlain(out)
	if err != nil {
		return raw
	}
	return string(filtered)
}

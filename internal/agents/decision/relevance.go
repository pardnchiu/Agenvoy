package decision

import (
	"context"
	"log/slog"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
)

const (
	unrelated        = "unrelated"
	minWithholdRunes = 1 << 12
)

var (
	relatedCriteria = map[string]any{
		"related": "Some part of `content` helps answer `request`.",
		unrelated: "Nothing in `content` bears on `request`.",
	}
	withheldKeys sync.Map
)

func Withhold(ctx context.Context, sessionID, key, objective, content string, attrs ...any) bool {
	if utf8.RuneCountInString(content) < minWithholdRunes {
		return false
	}
	seenKey := sessionID + "\x00" + key
	if _, seen := withheldKeys.Load(seenKey); seen {
		return false
	}
	if !Unrelated(ctx, objective, content, attrs...) {
		return false
	}
	withheldKeys.Store(seenKey, struct{}{})
	return true
}

func Unrelated(ctx context.Context, objective, content string, attrs ...any) bool {
	request, ok := Request(objective)
	if !ok {
		return false
	}
	agent, name, ok := Ready(ctx, attrs...)
	if !ok {
		return false
	}

	text := []rune(content)
	if limit := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request); len(text) > limit {
		text = text[:max(limit, 0)]
	}
	answers, err := Decide(ctx, agent, map[string]any{"request": request, "content": string(text)}, map[string]go_llm_router.DecisionQuestion{
		"related": {
			Type: go_llm_router.DecisionChoice,
			Instructions: map[string]any{
				"question": "Does `content` contain information that helps answer `request`?",
				"focus":    "Related when any part of `content` bears on `request`, even partly, as background, or as code the request may need to read or change. Unrelated only when `content` is about a different subject altogether.",
			},
			Criteria: relatedCriteria,
		},
	})
	if err != nil {
		slog.Debug("[Unavailable] relevance check: keep content",
			append(attrs, slog.String("model", name), slog.String("error", err.Error()))...)
		return false
	}
	return Confident(answers["related"], unrelated)
}

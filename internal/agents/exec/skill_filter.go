package exec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

const decisionSkillRelevant = "relevant"

var skillCriteria = map[string]any{
	decisionSkillRelevant: "The skill could carry out or support `request`, or `request` names it.",
	"unrelated":           "The skill is about a different kind of work than `request`.",
}

func unrelatedSkills(ctx context.Context, sessionID, objective string, scanner *runtime.SkillScanner, excludeSkills []string) []string {
	request, ok := decision.Request(objective)
	if !ok || scanner == nil {
		return nil
	}
	agent, model, ok := decision.Ready(ctx, slog.String("session", sessionID))
	if !ok {
		return nil
	}

	excluded := make(map[string]bool, len(excludeSkills))
	for _, name := range excludeSkills {
		excluded[strings.TrimSpace(name)] = true
	}
	var names []string
	for _, name := range scanner.List() {
		if !excluded[name] {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}

	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
	keep := make([]bool, len(names))
	var wg sync.WaitGroup
	for _, batch := range decision.Batch(len(names), budget, func(i int) int {
		return utf8.RuneCountInString(names[i]) + utf8.RuneCountInString(scanner.Skills.ByName[names[i]].Description)
	}) {
		wg.Go(func() {
			skills := make(map[string]map[string]string, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				key := fmt.Sprintf("s%d", i)
				skills[key] = map[string]string{"name": names[i], "description": scanner.Skills.ByName[names[i]].Description}
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": "Is skill `" + key + "` in `skills` relevant to `request`?",
						"focus":    "Judge by the skill's name and full description.",
					},
					Criteria: skillCriteria,
				}
			}
			answers, err := decision.Decide(ctx, agent, map[string]any{"request": request, "skills": skills}, questions)
			if err != nil {
				slog.Debug("[Unavailable] skill relevance check: keep batch",
					slog.String("session", sessionID),
					slog.String("model", model),
					slog.String("error", err.Error()))
				for _, i := range batch {
					keep[i] = true
				}
				return
			}
			for _, i := range batch {
				keep[i] = answers[fmt.Sprintf("s%d", i)].Choice == decisionSkillRelevant
			}
		})
	}
	wg.Wait()

	var list []string
	for i, name := range names {
		if !keep[i] {
			list = append(list, name)
		}
	}
	slog.Debug("skill relevance check",
		slog.String("session", sessionID),
		slog.Int("skills", len(names)),
		slog.Int("unrelated", len(list)))
	return list
}

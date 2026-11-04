package exec

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

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
	objective = strings.TrimSpace(objective)
	if objective == "" || scanner == nil || decision.Selected() == decision.Off {
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

	agent, model, err := decision.Agent(ctx)
	if err != nil {
		slog.Debug("decision.Agent",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
		return nil
	}

	request := go_pkg_utils.TruncateString(objective, configs.MAX_DECISION_RUNES)
	budget := configs.MAX_DECISION_STATE_RUNES - utf8.RuneCountInString(request)
	keep := make([]bool, len(names))
	var wg sync.WaitGroup
	for _, batch := range batchSkills(scanner, names, budget) {
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

func batchSkills(scanner *runtime.SkillScanner, names []string, budget int) [][]int {
	var batches [][]int
	var batch []int
	used := 0
	for i, name := range names {
		size := utf8.RuneCountInString(name) + utf8.RuneCountInString(scanner.Skills.ByName[name].Description)
		if len(batch) > 0 && (used+size > budget || len(batch) >= configs.MAX_DECISION_QUESTIONS) {
			batches = append(batches, batch)
			batch, used = nil, 0
		}
		batch = append(batch, i)
		used += size
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

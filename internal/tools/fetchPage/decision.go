package fetchPage

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/decision"
)

const decisionNoise = "noise"

var (
	blockSplitRegex = regexp.MustCompile(`\n[ \t]*\n`)
	titleRegex      = regexp.MustCompile(`(?m)^title: "(.*)"$`)
	noiseCriteria   = map[string]any{
		"keep":        "Page content a reader could need: article text, data, tables, figures, lists of facts or items that belong to the page's subject.",
		decisionNoise: "Page chrome that carries no information: ads, sponsored blocks, navigation or breadcrumb menus, share or subscribe prompts, cookie notices, 'read more' links, footers, related-article link lists.",
	}
)

func removeNoise(ctx context.Context, link, content string) string {
	agent, name, ok := decision.Ready(ctx, slog.String("url", link))
	if !ok {
		return content
	}

	blocks := blockSplitRegex.Split(strings.TrimSpace(content), -1)
	if len(blocks) < 2 {
		return content
	}

	noise := make([]bool, len(blocks))
	var wg sync.WaitGroup
	for _, batch := range decision.Batch(len(blocks), configs.MAX_DECISION_STATE_RUNES, func(i int) int { return utf8.RuneCountInString(blocks[i]) }) {
		wg.Go(func() {
			state := make(map[string]string, len(batch))
			questions := make(map[string]go_llm_router.DecisionQuestion, len(batch))
			for _, i := range batch {
				key := fmt.Sprintf("b%d", i)
				state[key] = blocks[i]
				questions[key] = go_llm_router.DecisionQuestion{
					Type: go_llm_router.DecisionChoice,
					Instructions: map[string]any{
						"question": "Is block `" + key + "` in `blocks` noise?",
						"focus":    "Judge the block by itself; when unsure, keep.",
					},
					Criteria: noiseCriteria,
				}
			}
			answers, err := decision.Decide(ctx, agent, map[string]any{"blocks": state}, questions)
			if err != nil {
				slog.Debug("[Unavailable] fetch_page noise check: keep batch",
					slog.String("url", link),
					slog.String("model", name),
					slog.String("error", err.Error()))
				return
			}
			for _, i := range batch {
				noise[i] = decision.Confident(answers[fmt.Sprintf("b%d", i)], decisionNoise)
			}
		})
	}
	wg.Wait()

	kept := make([]string, 0, len(blocks))
	for i, block := range blocks {
		if !noise[i] {
			kept = append(kept, block)
		}
	}
	slog.Debug("fetch_page noise filter",
		slog.String("url", link),
		slog.Int("blocks", len(blocks)),
		slog.Int("noise", len(blocks)-len(kept)))
	return strings.Join(kept, "\n\n")
}

func checkRelevance(ctx context.Context, objective, link, result string) string {
	if !decision.Unrelated(ctx, objective, result, slog.String("url", link)) {
		return result
	}

	title := ""
	if m := titleRegex.FindStringSubmatch(result); m != nil {
		title = m[1]
	}
	return fmt.Sprintf("Web page withheld: %s (title: %q) does not bear on the current request, so its content was not loaded. Use a different source.", link, title)
}

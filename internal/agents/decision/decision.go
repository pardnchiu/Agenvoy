package decision

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	"github.com/pardnchiu/go-llm-router/core/router"
	"github.com/pardnchiu/go-llm-router/core/typesafe"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var Providers = []string{"typesafe", "openrouter"}

// * can not use or input type not same as others, so hide first
var excludeModels = map[string]bool{
	"openrouter@cloudflare/clairic":       true,
	"openrouter@respan/span-01":           true,
	"openrouter@respan/span-01-lite":      true,
	"openrouter@respan/span-01-lite:free": true,
	"openrouter@scaledown/classify-1":     true,
}

const Off = ""

func Selected() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return Off
	}
	name := strings.TrimSpace(cfg.DecisionModel)
	if name == "off" {
		return Off
	}
	return name
}

func modelsCacheKey(name string) string {
	return "provider:models:decision:" + name
}

func DropModelsCache(ctx context.Context, name string) {
	torii.DropKeys(ctx, modelsCacheKey(name))
}

func Available(ctx context.Context) []string {
	found := make([][]string, len(Providers))

	var wg sync.WaitGroup
	for i, name := range Providers {
		wg.Go(func() {
			cfg, err := agentkeychain.Config(ctx, name+"@")
			if err != nil {
				return
			}
			base := go_llm_router.Config{APIKey: cfg.APIKey}
			models, err := torii.CachedList(ctx, modelsCacheKey(name), configs.TTL_MODELS_CACHE_SEC, func() ([]string, error) {
				if name == "typesafe" {
					return typesafe.Models(ctx, base)
				}
				return openrouter.Models(ctx, base, go_llm_router.ModelFilter{DecisionOnly: true})
			})
			if err != nil {
				slog.Debug("decision.Available",
					slog.String("provider", name),
					slog.String("error", err.Error()))
				return
			}
			for _, model := range models {
				if full := name + "@" + model; !excludeModels[full] {
					found[i] = append(found[i], full)
				}
			}
		})
	}
	wg.Wait()

	var out []string
	for _, list := range found {
		out = append(out, list...)
	}
	return out
}

func Agent(ctx context.Context) (go_llm_router.DecisionAgent, string, error) {
	name := Selected()
	if name == Off {
		return nil, "", utilsmsg.New(utilsmsg.Unavailable, "decision model", "off; set it in /model decision")
	}
	cfg, err := agentkeychain.Config(ctx, name)
	if err != nil {
		return nil, name, utilsmsg.Trace(name, "not configured", err)
	}
	agent, err := router.NewDecision(router.Config{Name: name, APIKey: cfg.APIKey})
	if err != nil {
		return nil, name, utilsmsg.Trace("router.NewDecision", name, err)
	}
	return agent, name, nil
}

func Ready(ctx context.Context, attrs ...any) (go_llm_router.DecisionAgent, string, bool) {
	agent, name, err := Agent(ctx)
	if err != nil {
		if name != "" {
			slog.Debug("decision.Agent", append(attrs, slog.String("error", err.Error()))...)
		}
		return nil, name, false
	}
	return agent, name, true
}

func Request(objective string) (string, bool) {
	objective = strings.TrimSpace(objective)
	if objective == "" {
		return "", false
	}
	return go_pkg_utils.TruncateString(objective, configs.MAX_DECISION_RUNES), true
}

func Decide(ctx context.Context, agent go_llm_router.DecisionAgent, state any, questions map[string]go_llm_router.DecisionQuestion) (map[string]go_llm_router.DecisionAnswer, error) {
	callCtx, cancel := context.WithTimeout(ctx, configs.TIMEOUT_DECISION_CALL)
	defer cancel()
	result, err := agent.Decide(callCtx, state, questions)
	if err != nil {
		return nil, err
	}
	return result.Answers, nil
}

func Confident(answer go_llm_router.DecisionAnswer, choice string) bool {
	return answer.Choice == choice && answer.Confidence != nil && *answer.Confidence >= configs.DECISION_MIN_CONFIDENCE
}

func Batch(n, budget int, size func(int) int) [][]int {
	var batches [][]int
	var batch []int
	used := 0
	for i := range n {
		one := size(i)
		if len(batch) > 0 && (used+one > budget || len(batch) >= configs.MAX_DECISION_QUESTIONS) {
			batches = append(batches, batch)
			batch, used = nil, 0
		}
		batch = append(batch, i)
		used += one
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

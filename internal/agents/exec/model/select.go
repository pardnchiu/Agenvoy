package model

import (
	"context"
	_ "embed"
	"log/slog"
	"slices"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/exec/retryHandler"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
)

type ConfigMeta struct {
	DefaultModel string                  `json:"default_model"`
	Models       []agenttypes.ModelEntry `json:"models"`
}

type SelectMeta struct {
	Agent            agenttypes.Agent
	ModelSet         agenttypes.ModelSet
	Task             string
	SkillDescription string
	SessionID        string
	ModelNames       []string
}

type selectConfigMeta struct {
	TagMap          map[string]string
	SelectionPrompt string
	JEVEnabled      bool
}

func selectModel(ctx context.Context, meta SelectMeta) ([]string, map[string]bool, string) {
	// * step 1: load model selection config
	cfg := loadConfig()
	deadMap := map[string]bool{}
	sessModel, sessReasoning := configbot.DefaultModel, configs.REASONING_AUTO
	if meta.SessionID != "" {
		sessModel, sessReasoning = configbot.GetModel(meta.SessionID)
	}
	isAutoReasoning := sessReasoning == configs.REASONING_AUTO

	if sessModel != configbot.DefaultModel {
		if _, ok := meta.ModelSet.Registry[sessModel]; ok {
			if isAutoReasoning {
				meta.ModelNames = []string{sessModel}
				_, sessReasoning, _ = runSelect(ctx, meta, cfg, deadMap)
			}
			return []string{sessModel}, deadMap, sessReasoning
		}
	}

	// * step 2: skip pass and cooling down, if only one model. skip select
	selectable := make(map[string]struct{}, len(meta.ModelSet.Order))
	avaiableModels := make([]string, 0, len(meta.ModelSet.Order))
	for _, e := range meta.ModelSet.Order {
		if cfg.TagMap[e.Name] == config.ModelTagPass {
			continue
		}
		selectable[e.Name] = struct{}{}

		if !retryHandler.IsCoolingDown(e.Name) {
			avaiableModels = append(avaiableModels, e.Name)
		}
	}
	if len(avaiableModels) <= 1 {
		if isAutoReasoning {
			meta.ModelNames = avaiableModels
			_, sessReasoning, _ = runSelect(ctx, meta, cfg, deadMap)
		}
		return avaiableModels, deadMap, sessReasoning
	}

	// * step 3: run select
	meta.ModelNames = avaiableModels
	modelSelected, reasoningSelected, pinned := runSelect(ctx, meta, cfg, deadMap)
	if isAutoReasoning {
		sessReasoning = reasoningSelected
	}

	// * step 4: merge select result
	modelNames := make([]string, 0, len(meta.ModelSet.Order))
	seen := map[string]bool{}
	for _, e := range modelSelected {
		if seen[e] {
			continue
		}
		if _, ok := selectable[e]; !ok {
			continue
		}
		if retryHandler.IsCoolingDown(e) {
			deadMap[e] = true
			continue
		}
		modelNames = append(modelNames, e)
		seen[e] = true
	}
	for _, e := range avaiableModels {
		if seen[e] || deadMap[e] {
			continue
		}
		modelNames = append(modelNames, e)
		seen[e] = true
	}

	// * step 5: keep pin head
	sticky := modelSelected[:min(pinned, len(modelSelected))]
	head := 0
	for head < len(modelNames) && slices.Contains(sticky, modelNames[head]) {
		head++
	}
	return append(modelNames[:head:head], config.OrderByRank(modelNames[head:])...), deadMap, sessReasoning
}

func runSelect(ctx context.Context, meta SelectMeta, cfgMeta selectConfigMeta, deadMap map[string]bool) ([]string, string, int) {
	if cfgMeta.JEVEnabled {
		jevCtx, cancel := context.WithTimeout(ctx, configs.TIMEOUT_JEV_CALL)
		newModelNames, reasoning, pinned, err := selectWithJEV(jevCtx, meta, cfgMeta.TagMap)
		cancel()
		if err == nil {
			return newModelNames, reasoning, pinned
		}
		if ctx.Err() == nil {
			slog.Debug("[Unavailable] jev: dispatch failed, falling back to LLM",
				slog.String("error", err.Error()))
		}
	}
	newModelNames, reasoning := selectWithLLM(ctx, meta, cfgMeta.SelectionPrompt, deadMap)
	return newModelNames, reasoning, 0
}

func loadConfig() selectConfigMeta {
	cfgMeta := selectConfigMeta{
		TagMap:          map[string]string{},
		SelectionPrompt: config.SelectionPrompt(&config.Config{}, false),
		JEVEnabled:      false,
	}
	if c, err := config.Load(); err == nil {
		cfgMeta.TagMap = c.ModelTag
		cfgMeta.SelectionPrompt = config.SelectionPrompt(c, false)
		cfgMeta.JEVEnabled = c.DispatcherBeta
	}
	return cfgMeta
}

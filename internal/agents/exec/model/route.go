package model

import (
	"context"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
	configbot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Route(ctx context.Context, assignModel, requestReasoning, requestTask, skillDesc, sessionID string) (agenttypes.Agent, []agenttypes.Agent, string, error) {
	// * if assign exist, then use it
	modelSet := agents.ModelSet()
	req := SelectMeta{
		Agent:            agents.DispatcherBot(),
		ModelSet:         modelSet,
		Task:             requestTask,
		SkillDescription: skillDesc,
		SessionID:        sessionID,
		Reasoning:        requestReasoning,
	}
	if assignModel != configbot.DefaultModel {
		agent, ok := modelSet.Registry[assignModel]
		if !ok || agent == nil {
			return nil, nil, "", utilsmsg.New(utilsmsg.NotFound, "model", "%q", assignModel)
		}

		if requestReasoning != "" && requestReasoning != configs.REASONING_AUTO {
			return agent, []agenttypes.Agent{}, requestReasoning, nil
		}

		// * check reasoning, if auto, then get fit
		reasoning := requestReasoning
		if reasoning == "" {
			_, reasoning = configbot.GetModel(sessionID)
		}
		if reasoning == configs.REASONING_AUTO {
			cfg := loadConfig()
			req.ModelNames = []string{assignModel}
			_, reasoning, _ = runSelect(ctx, req, cfg, map[string]bool{})
		}
		return agent, []agenttypes.Agent{}, reasoning, nil
	}

	// * if no assign, then get fit
	models, deadMap, reasoning := selectModel(ctx, req)
	var agent agenttypes.Agent
	for _, e := range models {
		if deadMap[e] {
			continue
		}
		if a, ok := modelSet.Registry[e]; ok && a != nil {
			agent, assignModel = a, e
			break
		}
	}

	// * get fallback
	cfg := loadConfig()
	fallbackAgents := make([]agenttypes.Agent, 0, len(modelSet.Order))
	for _, e := range modelSet.Order {
		if e.Name == assignModel || deadMap[e.Name] || cfg.TagMap[e.Name] == config.ModelTagPass {
			continue
		}
		if a, ok := modelSet.Registry[e.Name]; ok && a != nil {
			fallbackAgents = append(fallbackAgents, a)
		}
	}

	if agent == nil {
		if len(fallbackAgents) == 0 {
			return nil, nil, "", utilsmsg.New(utilsmsg.Unavailable, "model", "none available (dead: %d)", len(deadMap))
		}
		agent, fallbackAgents = fallbackAgents[0], fallbackAgents[1:]
	}
	return agent, fallbackAgents, reasoning, nil
}

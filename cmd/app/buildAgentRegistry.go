package main

import (
	"log/slog"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/claude"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/compat"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/copilot"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/deepseek"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/gemini"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/grok"
	grokoauth "github.com/pardnchiu/agenvoy/internal/agents/provider/grokOauth"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/nvidia"
	"github.com/pardnchiu/agenvoy/internal/agents/provider/openai"
	openrouter "github.com/pardnchiu/agenvoy/internal/agents/provider/openRouter"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/session/config"
)

func buildAgentRegistry() agentTypes.AgentRegistry {
	newFn := map[string]func(string) (agentTypes.Agent, error){
		"claude":     func(m string) (agentTypes.Agent, error) { return claude.New(m) },
		"openai":     func(m string) (agentTypes.Agent, error) { return openai.New(m) },
		"codex":      func(m string) (agentTypes.Agent, error) { return openaicodex.New(m) },
		"gemini":     func(m string) (agentTypes.Agent, error) { return gemini.New(m) },
		"grok":       func(m string) (agentTypes.Agent, error) { return grok.New(m) },
		"grok-oauth": func(m string) (agentTypes.Agent, error) { return grokoauth.New(m) },
		"copilot":    func(m string) (agentTypes.Agent, error) { return copilot.New(m) },
		"nvidia":     func(m string) (agentTypes.Agent, error) { return nvidia.New(m) },
		"deepseek":    func(m string) (agentTypes.Agent, error) { return deepseek.New(m) },
		"openrouter": func(m string) (agentTypes.Agent, error) { return openrouter.New(m) },
		"compat":     func(m string) (agentTypes.Agent, error) { return compat.New(m) },
	}

	agentEntries := exec.GetAgent()
	registry := agentTypes.AgentRegistry{
		Registry: make(map[string]agentTypes.Agent, len(agentEntries)),
		Entries:  make([]agentTypes.AgentEntry, 0, len(agentEntries)),
	}
	for _, e := range agentEntries {
		providerFull, _, _ := strings.Cut(e.Name, "@")
		prov, _, _ := strings.Cut(providerFull, "[")
		fn, ok := newFn[prov]
		if !ok {
			continue
		}
		a, err := fn(e.Name)
		if err != nil {
			slog.Warn("failed to initialize",
				slog.String("name", e.Name),
				slog.String("error", err.Error()))
			continue
		}
		registry.Registry[e.Name] = a
		registry.Entries = append(registry.Entries, e)
		if registry.Fallback == nil {
			registry.Fallback = a
		}
	}

	return registry
}

func dispatcherSelector(registry agentTypes.AgentRegistry) agentTypes.Agent {
	if cfg, err := config.Load(); err == nil && cfg.DispatcherModel != "" {
		if a, ok := registry.Registry[cfg.DispatcherModel]; ok {
			return a
		}
	}
	return registry.Fallback
}

func summarySelector(registry agentTypes.AgentRegistry) agentTypes.Agent {
	if cfg, err := config.Load(); err == nil && cfg.SummaryModel != "" {
		if a, ok := registry.Registry[cfg.SummaryModel]; ok {
			return a
		}
	}
	return nil
}

func refreshHost() (agentTypes.Agent, agentTypes.Agent, agentTypes.AgentRegistry) {
	registry := buildAgentRegistry()
	return dispatcherSelector(registry), summarySelector(registry), registry
}

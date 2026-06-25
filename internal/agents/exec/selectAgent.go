package exec

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	ProbeTimeout              = 15 * time.Second
	DispatcherCallTimeout     = 30 * time.Second
	UnresponsiveProbeInterval = 3 * time.Minute
	HealthCheckTimeout        = 10 * time.Second
)

type AgentConfig struct {
	SessionID    string                  `json:"session_id"`
	DefaultModel string                  `json:"default_model"`
	Models       []agentTypes.AgentEntry `json:"models"`
}

func GetAgent() []agentTypes.AgentEntry {
	cfg, err := go_pkg_filesystem.ReadJSON[AgentConfig](filesystem.ConfigPath)
	if err != nil || len(cfg.Models) == 0 {
		return []agentTypes.AgentEntry{}
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = cfg.Models[0].Name
	} else {
		for i, m := range cfg.Models {
			// * move default model to first be fallback
			if m.Name == cfg.DefaultModel {
				cfg.Models[0], cfg.Models[i] = cfg.Models[i], cfg.Models[0]
				break
			}
		}
	}
	return cfg.Models
}

func SelectAgentNames(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, userInput string, hasSkill bool, sessionID string) ([]string, map[string]bool) {
	dead := map[string]bool{}

	if sessionID != "" {
		model, reasoning := configBot.GetModel(sessionID)
		if reasoning != "" {
			provider.SetReasoningLevel(reasoning)
		}
		if model != "" && model != configBot.DefaultModel {
			if _, ok := registry.Registry[model]; ok {
				return []string{model}, dead
			}
		}
	}

	if len(registry.Entries) == 0 {
		return nil, dead
	}

	if len(registry.Entries) == 1 {
		return []string{registry.Entries[0].Name}, dead
	}

	registryOrder := make([]string, 0, len(registry.Entries))
	known := make(map[string]struct{}, len(registry.Entries))
	for _, e := range registry.Entries {
		registryOrder = append(registryOrder, e.Name)
		known[e.Name] = struct{}{}
	}

	picked := []string{}
	seen := map[string]bool{}

	bot = checkCooldown(bot, registry)

	if bot != nil {
		agentJson, err := json.Marshal(registry.Entries)
		if err == nil {
			userContent := strings.TrimSpace(userInput)
			if hasSkill {
				userContent = "[Run Skill] " + userContent
			}
			messages := []agentTypes.Message{
				{Role: "system", Content: strings.TrimSpace(configs.AgentSelector)},
				{Role: "user", Content: fmt.Sprintf("Available agents:\n%s\nUser request: %s", string(agentJson), userContent)},
			}
			prev := provider.GetReasoningLevel()
			provider.SetReasoningLevel("low")
			for range len(registry.Entries) {
				if ctx.Err() != nil {
					break
				}
				routingCtx, cancel := context.WithTimeout(ctx, DispatcherCallTimeout)
				resp, sendErr := bot.Send(routingCtx, messages, nil)
				cancel()
				if sendErr == nil {
					if resp != nil && len(resp.Choices) > 0 {
						if content, ok := resp.Choices[0].Message.Content.(string); ok {
							raw := strings.Trim(strings.TrimSpace(content), "\"'` \n")
							if raw != "" && raw != "NONE" {
								for n := range strings.SplitSeq(raw, ",") {
									n = strings.Trim(strings.TrimSpace(n), "\"'`")
									if n == "" || seen[n] {
										continue
									}
									if _, ok := known[n]; !ok {
										continue
									}
									if isCoolingDown(n) {
										dead[n] = true
										continue
									}
									picked = append(picked, n)
									seen[n] = true
								}
							}
						}
					}
					break
				}
				dead[bot.Name()] = true
				rl := isRateLimit(sendErr)
				if rl != nil {
					cooldownMap.Store(bot.Name(), rl.ResetsAt)
				}
				next := checkCooldown(nil, registry)
				hasNext := next != nil && !dead[next.Name()]
				if ctx.Err() == nil && rl == nil {
					slog.Warn("dispatcher routing failed",
						slog.String("name", bot.Name()),
						slog.String("error", sendErr.Error()))
				}
				if !hasNext {
					break
				}
				if ctx.Err() == nil && rl == nil {
					slog.Warn("dispatcher retrying with fallback",
						slog.String("name", next.Name()))
				}
				bot = next
			}
			provider.SetReasoningLevel(prev)
		}
	}

	for _, n := range registryOrder {
		if seen[n] || dead[n] {
			continue
		}
		picked = append(picked, n)
		seen[n] = true
	}
	return picked, dead
}

func SelectAgent(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, userInput string, hasSkill bool, sessionID string) agentTypes.Agent {
	names, dead := SelectAgentNames(ctx, bot, registry, userInput, hasSkill, sessionID)
	for _, n := range names {
		if dead[n] {
			continue
		}
		if a, ok := registry.Registry[n]; ok && a != nil {
			return a
		}
	}
	return registry.Fallback
}

const maxFallbackRounds = 3

func nextAgent(ctx context.Context, fallbacks *[]agentTypes.Agent, allAgents []agentTypes.Agent, round *int) (agentTypes.Agent, string) {
	for {
		agent, name := pickHealthyFallback(ctx, fallbacks)
		if agent != nil {
			return agent, name
		}
		*round++
		if *round >= maxFallbackRounds {
			return nil, ""
		}
		if ctx.Err() != nil {
			return nil, ""
		}
		slog.Warn("all agents failed, starting retry round",
			slog.Int("round", *round+1),
			slog.Int("max", maxFallbackRounds))
		rebuilt := make([]agentTypes.Agent, len(allAgents))
		copy(rebuilt, allAgents)
		*fallbacks = rebuilt
	}
}

func pickHealthyFallback(ctx context.Context, fallbacks *[]agentTypes.Agent) (agentTypes.Agent, string) {
	for len(*fallbacks) > 0 {
		cand := (*fallbacks)[0]
		*fallbacks = (*fallbacks)[1:]
		if cand == nil {
			continue
		}
		if utils.CheckAgentEndpointAlive(ctx, cand, HealthCheckTimeout) {
			return cand, cand.Name()
		}
		if ctx.Err() == nil {
			slog.Warn("fallback health check failed",
				slog.String("name", cand.Name()),
				slog.Duration("timeout", HealthCheckTimeout))
		}
	}
	return nil, ""
}

func ResolveAgent(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, userInput string, hasSkill bool, sessionID string) (agentTypes.Agent, []agentTypes.Agent, error) {
	names, dead := SelectAgentNames(ctx, bot, registry, userInput, hasSkill, sessionID)
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no agents available")
	}
	candidates := make([]agentTypes.Agent, 0, len(names))
	for _, n := range names {
		if dead[n] {
			continue
		}
		if a, ok := registry.Registry[n]; ok && a != nil {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no resolvable agents from %d names (dead: %d)", len(names), len(dead))
	}
	// * single candidate has no fallback; skip probe and let real Send drive retry/timeout
	if len(candidates) == 1 {
		return candidates[0], nil, nil
	}
	for i, a := range candidates {
		if utils.CheckAgentEndpointAlive(ctx, a, ProbeTimeout) {
			rest := make([]agentTypes.Agent, 0, len(candidates)-i-1)
			for _, b := range candidates[i+1:] {
				if dead[b.Name()] {
					continue
				}
				rest = append(rest, b)
			}
			return a, rest, nil
		}
		dead[a.Name()] = true
		if ctx.Err() == nil {
			slog.Warn("agent probe failed",
				slog.String("name", a.Name()),
				slog.Duration("timeout", ProbeTimeout))
		}
	}
	return nil, nil, fmt.Errorf("all %d candidates failed probe", len(candidates))
}

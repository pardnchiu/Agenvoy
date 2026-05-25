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
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

const (
	ProbeTimeout          = 5 * time.Second
	DispatcherCallTimeout = 10 * time.Second
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
		s := sessionManager.ReadStatus(sessionID)
		if s.Reasoning != "" {
			provider.SetReasoningLevel(s.Reasoning)
		}
		if s.Model != "" && s.Model != sessionManager.StatusModel {
			if _, ok := registry.Registry[s.Model]; ok {
				return []string{s.Model}, dead
			}
		}
	}

	if len(registry.Entries) == 0 {
		return nil, dead
	}

	registryOrder := make([]string, 0, len(registry.Entries))
	known := make(map[string]struct{}, len(registry.Entries))
	for _, e := range registry.Entries {
		registryOrder = append(registryOrder, e.Name)
		known[e.Name] = struct{}{}
	}

	picked := []string{}
	seen := map[string]bool{}

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
			routingCtx, cancel := context.WithTimeout(ctx, DispatcherCallTimeout)
			resp, sendErr := bot.Send(routingCtx, messages, nil)
			cancel()
			if sendErr == nil && len(resp.Choices) > 0 {
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
							picked = append(picked, n)
							seen[n] = true
						}
					}
				}
			} else if sendErr != nil {
				dead[bot.Name()] = true
				slog.Warn("dispatcher routing failed, falling back to registry order",
					slog.String("name", bot.Name()),
					slog.String("error", sendErr.Error()))
			}
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

func ProbeAgent(ctx context.Context, a agentTypes.Agent, timeout time.Duration) bool {
	if a == nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := a.Send(probeCtx, []agentTypes.Message{{Role: "user", Content: "."}}, nil)
	return err == nil
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
	for i, a := range candidates {
		if ProbeAgent(ctx, a, ProbeTimeout) {
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
		slog.Warn("agent probe failed",
			slog.String("name", a.Name()),
			slog.Duration("timeout", ProbeTimeout))
	}
	return nil, nil, fmt.Errorf("all %d candidates failed probe", len(candidates))
}

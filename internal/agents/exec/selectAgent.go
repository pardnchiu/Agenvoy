package exec

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func GetAgentEntries() []agentTypes.AgentEntry {
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return []agentTypes.AgentEntry{}
	}

	for _, dir := range configDir.Dirs {
		data, err := os.ReadFile(filepath.Join(dir, "config.json"))
		if err != nil {
			continue
		}
		var cfg struct {
			Models       []agentTypes.AgentEntry `json:"models"`
			DefaultModel string                  `json:"default_model"`
		}
		if json.Unmarshal(data, &cfg) != nil || len(cfg.Models) == 0 {
			continue
		}
		if cfg.DefaultModel != "" {
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
	return []agentTypes.AgentEntry{}
}

func SelectAgent(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, userInput string, hasSkill bool) agentTypes.Agent {
	trimInput := strings.TrimSpace(userInput)

	if len(registry.Entries) == 0 {
		return registry.Fallback
	}

	agentMap := make(map[string]struct{}, len(registry.Entries))
	for _, a := range registry.Entries {
		agentMap[a.Name] = struct{}{}
	}

	agentJson, err := json.Marshal(registry.Entries)
	if err != nil {
		return registry.Fallback
	}

	userContent := strings.TrimSpace(trimInput)
	if hasSkill {
		userContent = "[執行 Skill] " + userContent
	}

	messages := []agentTypes.Message{
		{
			Role:    "system",
			Content: strings.TrimSpace(configs.AgentSelector),
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"Available agents:\n%s\nUser request: %s",
				string(agentJson),
				userContent,
			),
		},
	}

	resp, err := bot.Send(ctx, messages, nil)
	if err != nil || len(resp.Choices) == 0 {
		return registry.Fallback
	}

	answer := ""
	if content, ok := resp.Choices[0].Message.Content.(string); ok {
		answer = strings.Trim(strings.TrimSpace(content), "\"'` \n")
	}

	if answer == "NONE" || answer == "" {
		return registry.Fallback
	}

	if _, ok := agentMap[answer]; ok {
		if a, ok := registry.Registry[answer]; ok {
			return a
		}
	}

	return registry.Fallback
}

package provider

import (
	_ "embed"
	"encoding/json"

	"github.com/pardnchiu/agenvoy/configs"
)

type ProviderItem struct {
	Default string               `json:"default"`
	Models  map[string]ModelItem `json:"models"`
}

type ModelItem struct {
	Description     string `json:"description"`
	NoTemperature   bool   `json:"no_temperature,omitempty"`
	ReasoningEffort bool   `json:"reasoning_effort,omitempty"` // OpenAI: supports reasoning_effort param
	ThinkingType    string `json:"thinking_type,omitempty"`    // Claude: "adaptive" | "enabled"
	ThinkingConfig  string `json:"thinking_config,omitempty"`  // Gemini: "budget" | "level"
}

var reasoningLevel = "medium"

func SetReasoningLevel(level string) {
	switch level {
	case "low", "high":
		reasoningLevel = level
	default:
		reasoningLevel = "medium"
	}
}

func GetReasoningLevel() string {
	return reasoningLevel
}

func SupportReasoningEffort(providerName, model string) bool {
	return Get(providerName, model).ReasoningEffort
}

func GetThinkingType(providerName, model string) string {
	return Get(providerName, model).ThinkingType
}

func GetThinkingConfig(providerName, model string) string {
	return Get(providerName, model).ThinkingConfig
}

func ThinkingBudget(level string) int {
	switch level {
	case "low":
		return 1024
	case "high":
		return 16384
	default:
		return 8192
	}
}

func parse(data []byte) ProviderItem {
	var cfg ProviderItem
	json.Unmarshal(data, &cfg)
	return cfg
}

func providers() map[string]ProviderItem {
	return map[string]ProviderItem{
		"claude":       parse(configs.ClaudeModels),
		"codex":        parse(configs.CodexModels),
		"copilot":      parse(configs.CopilotModels),
		"gemini":       parse(configs.GeminiModels),
		"nvidia":       parse(configs.NvidiaModels),
		"openai":       parse(configs.OpenaiModels),
	}
}

func Default(provider string) string {
	return providers()[provider].Default
}

func Get(provider, model string) ModelItem {
	cfg, exist := providers()[provider]
	if !exist {
		return ModelItem{}
	}

	if info, exist := cfg.Models[model]; exist {
		return info
	}

	if info, exist := cfg.Models[cfg.Default]; exist {
		return info
	}
	return ModelItem{}
}

func Models(provider string) map[string]ModelItem {
	cfg, exist := providers()[provider]
	if !exist {
		return nil
	}
	return cfg.Models
}

func SupportTemperature(providerName, model string) bool {
	return !Get(providerName, model).NoTemperature
}

package provider

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
)

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

func parse(data []byte) map[string]ModelItem {
	var m map[string]ModelItem
	json.Unmarshal(data, &m)
	return m
}

func providers() map[string]map[string]ModelItem {
	return map[string]map[string]ModelItem{
		"claude":  parse(configs.ClaudeModels),
		"codex":   parse(configs.CodexModels),
		"copilot": parse(configs.CopilotModels),
		"gemini":  parse(configs.GeminiModels),
		"nvidia":  parse(configs.NvidiaModels),
		"openai":  parse(configs.OpenaiModels),
	}
}

func Get(provider, model string) ModelItem {
	models, exist := providers()[provider]
	if !exist {
		return ModelItem{}
	}
	if info, exist := models[model]; exist {
		return info
	}
	return ModelItem{}
}

func Models(provider string) map[string]ModelItem {
	return providers()[provider]
}

func SupportTemperature(providerName, model string) bool {
	return !Get(providerName, model).NoTemperature
}

func NewHTTPClient() *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	return &http.Client{
		Timeout:   5 * time.Minute,
		Transport: transport,
	}
}

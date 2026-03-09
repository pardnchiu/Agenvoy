package provider

import (
	_ "embed"
	"encoding/json"
)

//go:embed embed/claude.json
var claudeJosn []byte

//go:embed embed/copilot.json
var copilotJson []byte

//go:embed embed/gemini.json
var geminiJson []byte

//go:embed embed/nvidia.json
var nvidiaJson []byte

//go:embed embed/openai.json
var openaiJson []byte

type ProviderItem struct {
	Default string               `json:"default"`
	Models  map[string]ModelItem `json:"models"`
}

type ModelItem struct {
	Input       int    `json:"input"`
	Output      int    `json:"output"`
	Description string `json:"description"`
}

func parse(data []byte) ProviderItem {
	var cfg ProviderItem
	json.Unmarshal(data, &cfg)
	return cfg
}

func configs() map[string]ProviderItem {
	return map[string]ProviderItem{
		"claude":  parse(claudeJosn),
		"copilot": parse(copilotJson),
		"gemini":  parse(geminiJson),
		"nvidia":  parse(nvidiaJson),
		"openai":  parse(openaiJson),
	}
}

func Default(provider string) string {
	return configs()[provider].Default
}

func Get(provider, model string) ModelItem {
	cfg, exist := configs()[provider]
	if !exist {
		return ModelItem{Input: 128000, Output: 16384}
	}

	if info, exist := cfg.Models[model]; exist {
		return info
	}

	if info, exist := cfg.Models[cfg.Default]; exist {
		return info
	}
	return ModelItem{Input: 128000, Output: 16384}
}

func Models(provider string) map[string]ModelItem {
	cfg, exist := configs()[provider]
	if !exist {
		return nil
	}
	return cfg.Models
}

func InputBytes(provider, model string) int {
	return Get(provider, model).Input * 4
}

func OutputTokens(provider, model string) int {
	return Get(provider, model).Output
}

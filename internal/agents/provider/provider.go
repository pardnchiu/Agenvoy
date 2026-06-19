package provider

import (
	"net/http"
	"strings"
	"time"
)

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

func SupportTemperature(providerName, model string) bool {
	switch providerName {
	case "openai", "copilot", "codex":
		if strings.HasPrefix(model, "gpt-5") {
			return false
		}
	case "deepseek":
		if model == "deepseek-reasoner" {
			return false
		}
	case "claude":
		return false
	case "gemini":
		if strings.Contains(model, "-preview") {
			return false
		}
	}
	return true
}

func SupportReasoningEffort(providerName, model string) bool {
	switch providerName {
	case "openai", "copilot":
		if !strings.HasPrefix(model, "gpt-5") {
			return false
		}
		if strings.Contains(model, "-codex") || strings.HasSuffix(model, "-pro") {
			return false
		}
		return true
	case "grok", "grok-oauth":
		return strings.Contains(model, "-mini")
	}
	return false
}

func GetThinkingType(providerName, model string) string {
	if providerName != "claude" {
		return ""
	}
	if strings.Contains(model, "-20") {
		return "enabled"
	}
	return "adaptive"
}

func GetThinkingConfig(providerName, model string) string {
	if providerName != "gemini" {
		return ""
	}
	if strings.HasPrefix(model, "gemini-2.5-") {
		return "budget"
	}
	if strings.HasPrefix(model, "gemini-3") {
		return "level"
	}
	return ""
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

func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute}
}

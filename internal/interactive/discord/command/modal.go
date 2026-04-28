package discordCommand

import (
	"fmt"

	"github.com/pardnchiu/go-utils/filesystem/keychain"
)

func modalEnvKey(customID string) string {
	switch customID {
	case "modal_add-gemini":
		return "GEMINI_API_KEY"
	case "modal_add-openai":
		return "OPENAI_API_KEY"
	case "modal_add-claude":
		return "ANTHROPIC_API_KEY"
	case "modal_add-nim":
		return "NVIDIA_API_KEY"
	default:
		return ""
	}
}

func ModalHandler(customID, apiKey string) string {
	envKey := modalEnvKey(customID)
	if envKey == "" {
		return "Unknown modal"
	}
	if apiKey == "" {
		return "API key is required"
	}
	if err := keychain.Set(envKey, apiKey); err != nil {
		return fmt.Sprintf("keychain.Set %s: %s", envKey, err.Error())
	}
	return fmt.Sprintf("saved %s", envKey)
}

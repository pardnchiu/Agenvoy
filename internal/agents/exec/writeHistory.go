package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func writeHistory(choice agentTypes.OutputChoices, configDir *utils.ConfigDirData, session *agentTypes.AgentSession) error {
	session.Histories = append(session.Histories, choice.Message)

	filtered := make([]agentTypes.Message, 0, len(session.Histories))
	for _, m := range session.Histories {
		if m.Role == "system" {
			continue
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			continue
		}
		if m.Role == "tool" {
			continue
		}
		filtered = append(filtered, m)
	}

	sessionDir := filepath.Join(configDir.Home, session.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("os.MkdirAll: %w", err)
	}
	historyPath := filepath.Join(sessionDir, "history.json")
	historyData, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if err := os.WriteFile(historyPath, historyData, 0644); err != nil {
		return fmt.Errorf("os.WriteFile: %w", err)
	}
	return nil
}

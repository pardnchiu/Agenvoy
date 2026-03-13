package exec

import (
	"encoding/json"
	"fmt"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func writeHistory(choice agentTypes.OutputChoices, session *agentTypes.AgentSession) error {
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
	historyData, err := json.Marshal(filtered)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	err = filesystem.SaveHistory(session.ID, string(historyData))
	if err != nil {
		return fmt.Errorf("filesystem.SaveHistory: %w", err)
	}
	return nil
	// sessionDir := filepath.Join(filesystem.SessionsDir, session.ID)
	// if err := os.MkdirAll(sessionDir, 0755); err != nil {
	// 	return fmt.Errorf("os.MkdirAll: %w", err)
	// }
	// historyPath := filepath.Join(sessionDir, "history.json")
	// if err != nil {
	// 	return fmt.Errorf("json.Marshal: %w", err)
	// }
	// if err := filesystem.WriteFile(historyPath, string(historyData), 0644); err != nil {
	// 	return fmt.Errorf("utils.WriteFile: %w", err)
	// }
	// return nil
}

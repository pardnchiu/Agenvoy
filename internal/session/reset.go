package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

func ResetHistoryKeepSummary(sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}
	sessionDir := filesystem.SessionDir(sessionID)

	if err := os.Remove(filesystem.HistoryPath(sessionID)); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("os.Remove history.json: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(sessionDir, "tool_calls")); err != nil {
		return 0, fmt.Errorf("os.RemoveAll tool_calls: %w", err)
	}
	if err := os.Remove(filesystem.ActionLogPath(sessionID)); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("os.Remove action.log: %w", err)
	}

	db := torii.DB(torii.DBSessionHist)
	keys := db.Keys(sessionID + ":*")
	if len(keys) == 0 {
		return 0, nil
	}
	return db.Del(keys...), nil
}

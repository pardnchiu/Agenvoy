package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
)

func Reset(sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("sessionID is required")
	}
	sessionDir := filesystem.SessionDir(sessionID)

	if err := os.Remove(filesystem.HistoryPath(sessionID)); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("os.Remove [%s]: %w", filesystem.HistoryPath(sessionID), err)
	}

	if err := os.RemoveAll(filepath.Join(sessionDir, "history")); err != nil {
		return 0, fmt.Errorf("os.RemoveAll [%s]: %w", filepath.Join(sessionDir, "history"), err)
	}

	os.RemoveAll(filesystem.PendingDir(sessionID))

	if err := os.Remove(filesystem.ActionLogPath(sessionID)); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("os.Remove [%s]: %w", filesystem.ActionLogPath(sessionID), err)
	}

	historyStore.Clear(sessionID)
	sessionHistory.ClearMutex(sessionID)

	db := torii.DB(torii.DBSessionHist)
	keys := db.Keys(sessionID + ":*")
	if len(keys) == 0 {
		return 0, nil
	}
	return db.Del(keys...), nil
}

package history

import (
	"encoding/json"
	"fmt"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
)

func Replace(sessionID string, messages []agentTypes.Message) error {
	if sessionID == "" {
		return fmt.Errorf("session id is required")
	}

	mu, _ := muMap.LoadOrStore(sessionID, &sync.Mutex{})
	lock := mu.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	historyPath := filesystem.HistoryPath(sessionID)

	raw, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("json: Marshal: %w", err)
	}
	if err := go_pkg_filesystem.WriteFile(historyPath, string(raw), 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/agenvoy/internal/filesystem: WriteFile: %w", err)
	}

	db := torii.DB(torii.DBSessionHist)
	keys := db.Keys(sessionID + ":*")
	if len(keys) > 0 {
		db.Del(keys...)
	}

	return nil
}

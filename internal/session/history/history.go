package history

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
)

var muMap sync.Map

func Append(sessionID string, delta []agentTypes.Message) error {
	if sessionID == "" || len(delta) == 0 {
		return nil
	}

	mu, _ := muMap.LoadOrStore(sessionID, &sync.Mutex{})
	lock := mu.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	historyPath := filesystem.HistoryPath(sessionID)

	latest, err := go_pkg_filesystem.ReadJSON[[]agentTypes.Message](historyPath)
	if err != nil {
		latest = nil
	}
	latest = append(latest, delta...)

	raw, err := json.Marshal(latest)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if err := go_pkg_filesystem.WriteFile(historyPath, string(raw), 0644); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem WriteFile: %w", err)
	}

	if historyStore.IsReady() && !historyStore.IsExist(sessionID) && len(latest) > len(delta) {
		if err := historyStore.Write(sessionID, latest); err != nil {
			slog.Warn("historyStore Write",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	} else {
		if err := historyStore.Write(sessionID, delta); err != nil {
			slog.Warn("historyStore Write",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		}
	}

	if filesystem.MaxHistoryBytes > 0 && len(raw) > filesystem.MaxHistoryBytes {
		compact(sessionID, historyPath, latest, len(raw))
	}

	return nil
}

func ClearMutex(sessionID string) {
	muMap.Delete(sessionID)
}

func Get(sessionID string) (old, max []agentTypes.Message) {
	historyPath := filesystem.HistoryPath(sessionID)
	oldHistory, err := go_pkg_filesystem.ReadJSON[[]agentTypes.Message](historyPath)
	if err != nil {
		return nil, nil
	}

	maxHistory := oldHistory
	if len(oldHistory) > filesystem.MaxHistoryMessages {
		maxHistory = oldHistory[len(oldHistory)-filesystem.MaxHistoryMessages:]
	}
	return oldHistory, maxHistory
}

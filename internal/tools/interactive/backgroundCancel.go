package interactive

import (
	"context"
	"errors"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var ErrBackgroundCanceled = errors.New("[Canceled] background task: the owning task was canceled")

var (
	backgroundCancelMu sync.Mutex
	backgroundCancels  = map[string]map[string]context.CancelFunc{}
	backgroundCanceled = map[string]bool{}
)

func backgroundKey(sessionID, taskHash string) string {
	return sessionID + "/" + taskHash
}

func RegisterBackgroundCancel(sessionID, taskHash, id string, cancel context.CancelFunc) {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	if backgroundCancels[key] == nil {
		backgroundCancels[key] = map[string]context.CancelFunc{}
	}
	backgroundCancels[key][id] = cancel
}

func unregisterBackgroundCancel(sessionID, taskHash, id string) {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	delete(backgroundCancels[key], id)
	if len(backgroundCancels[key]) == 0 {
		delete(backgroundCancels, key)
	}
}

func CancelTask(sessionID, taskHash string) {
	StopBackground(sessionID, taskHash)
	DeletePending(sessionID, taskHash)
}

func PauseTask(sessionID, taskHash string) {
	StopBackground(sessionID, taskHash)
}

func StopBackground(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}

	backgroundCancelMu.Lock()
	key := backgroundKey(sessionID, taskHash)
	list := make([]context.CancelFunc, 0, len(backgroundCancels[key]))
	for _, cancel := range backgroundCancels[key] {
		list = append(list, cancel)
	}
	if len(list) > 0 {
		backgroundCanceled[key] = true
	}
	backgroundCancelMu.Unlock()

	for _, cancel := range list {
		cancel()
	}

	pendingMu.Lock()
	defer pendingMu.Unlock()
	meta, err := readPending(filesystem.PendingMetaPath(sessionID, taskHash))
	if err != nil || len(meta.Background) == 0 {
		return
	}
	meta.Background = nil
	writePending(sessionID, taskHash, &meta)
}

func takeBackgroundCanceled(sessionID, taskHash string) bool {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	if !backgroundCanceled[key] {
		return false
	}
	if len(backgroundCancels[key]) == 0 {
		delete(backgroundCanceled, key)
	}
	return true
}

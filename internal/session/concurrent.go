package session

import (
	"context"
	"sync"

	"github.com/pardnchiu/agenvoy/configs"
)

var (
	concurrentMu    sync.Mutex
	concurrentSlots = make(map[string]chan struct{})
	concurrentMap   = make(map[string]bool)
)

func ClaimIdle(sessionID string) bool {
	concurrentMu.Lock()
	defer concurrentMu.Unlock()

	if concurrentMap[sessionID] {
		return false
	}

	slot, ok := concurrentSlots[sessionID]
	if ok && len(slot) > 0 {
		return false
	}
	concurrentMap[sessionID] = true
	return true
}

func AddConcurrent(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	concurrentMu.Lock()
	slot, ok := concurrentSlots[sessionID]
	if !ok {
		slot = make(chan struct{}, configs.MAX_SESSION_TASKS)
		concurrentSlots[sessionID] = slot
	}
	select {
	case slot <- struct{}{}:
		delete(concurrentMap, sessionID)
		concurrentMu.Unlock()
		return nil
	default:
	}
	concurrentMu.Unlock()

	select {
	case slot <- struct{}{}:
		concurrentMu.Lock()
		delete(concurrentMap, sessionID)
		concurrentMu.Unlock()
		return nil
	case <-ctx.Done():
		concurrentMu.Lock()
		delete(concurrentMap, sessionID)
		concurrentMu.Unlock()
		return ctx.Err()
	}
}

func RemoveConcurrent(sessionID string) {
	if sessionID == "" {
		return
	}
	concurrentMu.Lock()
	delete(concurrentMap, sessionID)
	slot, ok := concurrentSlots[sessionID]
	concurrentMu.Unlock()
	if !ok {
		return
	}
	select {
	case <-slot:
	default:
	}
}

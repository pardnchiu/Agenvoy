package session

import (
	"context"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	concurrentMu    sync.Mutex
	concurrentSlots = make(map[string]chan struct{})
	claimed         = make(map[string]bool)
)

func ClaimIdle(sessionID string) bool {
	concurrentMu.Lock()
	defer concurrentMu.Unlock()
	if claimed[sessionID] {
		return false
	}
	slot, ok := concurrentSlots[sessionID]
	if ok && len(slot) > 0 {
		return false
	}
	claimed[sessionID] = true
	return true
}

func AddConcurrent(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	concurrentMu.Lock()
	delete(claimed, sessionID)
	slot, ok := concurrentSlots[sessionID]
	if !ok {
		slot = make(chan struct{}, filesystem.MaxSessionTasks)
		concurrentSlots[sessionID] = slot
	}
	concurrentMu.Unlock()

	select {
	case slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func RemoveConcurrent(sessionID string) {
	if sessionID == "" {
		return
	}
	concurrentMu.Lock()
	delete(claimed, sessionID)
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

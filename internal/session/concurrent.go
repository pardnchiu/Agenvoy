package session

import (
	"context"
	"sync"

	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

const (
	defaultMaxConcurrentPerSession = 3
	hardCapMaxConcurrentPerSession = 10
)

var MaxConcurrentPerSession = max(defaultMaxConcurrentPerSession,
	min(hardCapMaxConcurrentPerSession,
		go_pkg_utils.GetWithDefaultInt("MAX_SESSION_TASKS", defaultMaxConcurrentPerSession)))

var (
	concurrentMu    sync.Mutex
	concurrentSlots = make(map[string]chan struct{})
)

func AddConcurrent(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	concurrentMu.Lock()
	slot, ok := concurrentSlots[sessionID]
	if !ok {
		slot = make(chan struct{}, MaxConcurrentPerSession)
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

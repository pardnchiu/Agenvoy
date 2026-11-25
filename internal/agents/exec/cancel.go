package exec

import (
	"context"
	"sync"
)

var (
	cancelFnMap   = map[string]context.CancelCauseFunc{}
	cancelFnMapMu sync.Mutex
)

func Cancel(taskHash string, cause error) bool {
	cancelFnMapMu.Lock()
	fn, ok := cancelFnMap[taskHash]
	cancelFnMapMu.Unlock()
	if !ok {
		return false
	}

	fn(cause)
	return true
}

func registerCancel(taskHash string, cancel context.CancelCauseFunc) {
	if taskHash == "" {
		return
	}
	cancelFnMapMu.Lock()
	cancelFnMap[taskHash] = cancel
	cancelFnMapMu.Unlock()
}

func unregisterCancel(taskHash string) {
	if taskHash == "" {
		return
	}
	cancelFnMapMu.Lock()
	delete(cancelFnMap, taskHash)
	cancelFnMapMu.Unlock()
}

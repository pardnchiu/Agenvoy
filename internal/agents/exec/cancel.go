package exec

import (
	"context"
	"sync"

	agentsexechook "github.com/pardnchiu/agenvoy/internal/agents/exec/hook"
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

	if pass, _ := agentsexechook.Run(context.Background(), agentsexechook.Load(""), agentsexechook.BeforeTaskCancel, agentsexechook.Meta{TaskHash: taskHash, Err: cause}); !pass {
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

package runtime

import (
	"errors"
	"sync"
)

var ErrUserCanceled = errors.New("user cancelled")

var ErrUserAborted = errors.New("[Canceled] task: aborted by user; pending kept for resume")

var (
	cancelMu       sync.RWMutex
	cancelNotifier func(sessionID, taskHash, reason string)
)

func RegisterCancelNotifier(fn func(sessionID, taskHash, reason string)) {
	cancelMu.Lock()
	defer cancelMu.Unlock()
	cancelNotifier = fn
}

func NotifyCanceled(sessionID, taskHash, reason string) {
	cancelMu.RLock()
	fn := cancelNotifier
	cancelMu.RUnlock()
	if fn == nil || sessionID == "" {
		return
	}
	fn(sessionID, taskHash, reason)
}

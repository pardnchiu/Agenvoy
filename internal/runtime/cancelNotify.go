package runtime

import (
	"context"
	"errors"
	"sync"
)

type userCanceledError struct{}

func (userCanceledError) Error() string { return "user cancelled" }

func (userCanceledError) Is(target error) bool { return target == context.Canceled }

var ErrUserCanceled error = userCanceledError{}

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

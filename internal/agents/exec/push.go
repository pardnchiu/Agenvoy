package exec

import (
	"context"
	"strings"
	"sync"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

type PushPayload struct {
	SessionID string
	Text      string
	Model     string
	Usage     *agentTypes.Usage
	Duration  time.Duration
	Prefix    string
}

type PushFunc func(ctx context.Context, payload PushPayload)

var (
	pushHookMu sync.RWMutex
	pushHooks  = map[string]PushFunc{}
)

func RegisterPushHook(prefix string, fn PushFunc) {
	pushHookMu.Lock()
	pushHooks[prefix] = fn
	pushHookMu.Unlock()
}

func lookupPushHook(sid string) (PushFunc, bool) {
	pushHookMu.RLock()
	defer pushHookMu.RUnlock()
	for prefix, fn := range pushHooks {
		if strings.HasPrefix(sid, prefix) {
			return fn, true
		}
	}
	return nil, false
}

type pushSuppressKey struct{}

func SuppressDcPush(ctx context.Context) context.Context {
	return context.WithValue(ctx, pushSuppressKey{}, true)
}

func isDcPushSuppressed(ctx context.Context) bool {
	v, _ := ctx.Value(pushSuppressKey{}).(bool)
	return v
}

type pushPrefixKey struct{}

func WithDcPushPrefix(ctx context.Context, prefix string) context.Context {
	return context.WithValue(ctx, pushPrefixKey{}, prefix)
}

func dcPushPrefix(ctx context.Context) string {
	v, _ := ctx.Value(pushPrefixKey{}).(string)
	return v
}

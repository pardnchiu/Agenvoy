package agents

import (
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type RefreshFunc func() (agentTypes.Agent, agentTypes.AgentRegistry)

var (
	mu         sync.RWMutex
	dispatcher agentTypes.Agent
	registry   agentTypes.AgentRegistry
	scanner    *runtime.SkillScanner
	refresher  RefreshFunc
)

func Set(d agentTypes.Agent, r agentTypes.AgentRegistry, s *runtime.SkillScanner) {
	mu.Lock()
	defer mu.Unlock()
	dispatcher = d
	registry = r
	scanner = s
}

func SetRefresher(fn RefreshFunc) {
	mu.Lock()
	defer mu.Unlock()
	refresher = fn
}

func Reload() bool {
	mu.RLock()
	fn := refresher
	mu.RUnlock()
	if fn == nil {
		return false
	}
	d, r := fn()
	mu.Lock()
	dispatcher = d
	registry = r
	mu.Unlock()
	return true
}

func Dispatcher() agentTypes.Agent {
	mu.RLock()
	defer mu.RUnlock()
	return dispatcher
}

func Registry() agentTypes.AgentRegistry {
	mu.RLock()
	defer mu.RUnlock()
	return registry
}

func Scanner() *runtime.SkillScanner {
	mu.RLock()
	defer mu.RUnlock()
	return scanner
}

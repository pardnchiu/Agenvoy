package agents

import (
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type RefreshFunc func() (agentTypes.Agent, agentTypes.Agent, agentTypes.AgentRegistry)

var (
	mu         sync.RWMutex
	dispatcher agentTypes.Agent
	summary    agentTypes.Agent
	registry   agentTypes.AgentRegistry
	scanner    *runtime.SkillScanner
	refresher  RefreshFunc
)

func Set(dispatcherBot agentTypes.Agent, summaryBot agentTypes.Agent, agentRegistry agentTypes.AgentRegistry, skillScanner *runtime.SkillScanner) {
	mu.Lock()
	defer mu.Unlock()

	dispatcher = dispatcherBot
	summary = summaryBot
	registry = agentRegistry
	scanner = skillScanner
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

	dispatcherBot, summaryBot, agentRegistry := fn()
	mu.Lock()
	dispatcher = dispatcherBot
	summary = summaryBot
	registry = agentRegistry
	mu.Unlock()
	return true
}

func DispatcherBot() agentTypes.Agent {
	mu.RLock()
	defer mu.RUnlock()
	return dispatcher
}

func SummaryBot() agentTypes.Agent {
	mu.RLock()
	defer mu.RUnlock()
	return summary
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

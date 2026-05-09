package host

import (
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

type RefreshFunc func() (agentTypes.Agent, agentTypes.AgentRegistry)

var (
	mu        sync.RWMutex
	planner   agentTypes.Agent
	registry  agentTypes.AgentRegistry
	scanner   *skill.SkillScanner
	refresher RefreshFunc
)

func Set(p agentTypes.Agent, r agentTypes.AgentRegistry, s *skill.SkillScanner) {
	mu.Lock()
	defer mu.Unlock()
	planner = p
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
	p, r := fn()
	mu.Lock()
	planner = p
	registry = r
	mu.Unlock()
	return true
}

func Planner() agentTypes.Agent {
	mu.RLock()
	defer mu.RUnlock()
	return planner
}

func Registry() agentTypes.AgentRegistry {
	mu.RLock()
	defer mu.RUnlock()
	return registry
}

func Scanner() *skill.SkillScanner {
	mu.RLock()
	defer mu.RUnlock()
	return scanner
}

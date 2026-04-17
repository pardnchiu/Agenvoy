package host

import (
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

var (
	mu       sync.RWMutex
	planner  agentTypes.Agent
	registry agentTypes.AgentRegistry
	scanner  *skill.SkillScanner
)

func Set(p agentTypes.Agent, r agentTypes.AgentRegistry, s *skill.SkillScanner) {
	mu.Lock()
	defer mu.Unlock()
	planner = p
	registry = r
	scanner = s
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

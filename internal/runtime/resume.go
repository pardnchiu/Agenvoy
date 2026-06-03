package runtime

import (
	"strings"
	"sync"
)

type resumeEntry struct {
	prefix  string
	handler func(sessionID, taskHash string, answers []any)
}

var (
	resumeMu       sync.RWMutex
	resumeHandlers []resumeEntry
)

func RegisterResumeHandler(prefix string, fn func(string, string, []any)) {
	resumeMu.Lock()
	defer resumeMu.Unlock()
	for i, e := range resumeHandlers {
		if e.prefix == prefix {
			resumeHandlers[i].handler = fn
			return
		}
	}
	resumeHandlers = append(resumeHandlers, resumeEntry{prefix: prefix, handler: fn})
}

func TriggerResume(sessionID, taskHash string, answers []any) bool {
	resumeMu.RLock()
	defer resumeMu.RUnlock()

	best := -1
	bestLen := -1
	for i, e := range resumeHandlers {
		if e.prefix != "" && strings.HasPrefix(sessionID, e.prefix) && len(e.prefix) > bestLen {
			best = i
			bestLen = len(e.prefix)
		}
	}
	if best == -1 {
		for i, e := range resumeHandlers {
			if e.prefix == "" {
				best = i
				break
			}
		}
	}
	if best == -1 {
		return false
	}
	go resumeHandlers[best].handler(sessionID, taskHash, answers)
	return true
}

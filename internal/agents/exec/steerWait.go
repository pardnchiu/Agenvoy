package exec

import (
	"strings"
	"sync"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

var (
	steerMap   sync.Map
	runningMap sync.Map
	runningMu  sync.Mutex
)

type steerEntry struct {
	mu   sync.Mutex
	list []string
}

type runningEntry struct {
	count int
}

func markRunning(sessionID string) func() {
	if sessionID == "" {
		return func() {}
	}
	v, _ := runningMap.LoadOrStore(sessionID, &runningEntry{})
	e := v.(*runningEntry)
	runningMu.Lock()
	e.count++
	runningMu.Unlock()

	return func() {
		runningMu.Lock()
		e.count--
		done := e.count <= 0
		runningMu.Unlock()
		if done {
			runningMap.Delete(sessionID)
		}
	}
}

func IsRunning(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	v, ok := runningMap.Load(sessionID)
	if !ok {
		return false
	}
	runningMu.Lock()
	defer runningMu.Unlock()
	return v.(*runningEntry).count > 0
}

func AppendSteer(sessionID, windowHash, text string) {
	v, _ := steerMap.LoadOrStore(sessionID, &steerEntry{})
	e := v.(*steerEntry)
	e.mu.Lock()
	e.list = append(e.list, text)
	e.mu.Unlock()
	sessionlog.Steer(sessionID, windowHash, text)
}

func getSteer(sessionID string) []string {
	v, ok := steerMap.Load(sessionID)
	if !ok {
		return nil
	}
	e := v.(*steerEntry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.list) == 0 {
		return nil
	}
	out := e.list
	e.list = nil
	return out
}

func ClearSteer(sessionID string) {
	steerMap.Delete(sessionID)
}

func formatSteerInjection(pending []string) string {
	return strings.ReplaceAll(strings.TrimSpace(configs.InjectSteer), "{{.Messages}}", strings.Join(pending, "\n"))
}

func injectBackgroundReport(session *agenttypes.Session, parentTask string, events chan<- agenttypes.Event) bool {
	report := interactive.TakeBackgroundReport(session.ID, parentTask)
	if report == "" {
		return false
	}
	session.ToolHistories = append(session.ToolHistories, go_llm_router.Message{Role: "user", Content: report})
	header, _, _ := strings.Cut(report, "\n")
	events <- agenttypes.Event{Type: agenttypes.EventUserInjected, Text: header}
	return true
}

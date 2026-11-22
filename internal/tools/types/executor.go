package toolTypes

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/internal/runtime"
)

type ScriptToolExecutor interface {
	IsExist(name string) bool
	Execute(ctx context.Context, name string, args json.RawMessage, workDir string) (string, error)
	GetTools() []map[string]any
}

type Executor struct {
	ToolsMu          sync.Mutex
	WorkDir          string
	SessionID        string
	Tools            []go_llm_router.Tool
	AllTools         []go_llm_router.Tool
	UnmarkedTools    map[string]bool
	ExcludeTools     map[string]bool
	ScriptToolbox    ScriptToolExecutor
	ExtScriptToolbox ScriptToolExecutor

	SkillScanner    *runtime.SkillScanner
	CancelExecution context.CancelFunc
	PendingTask     string
	BackgroundGroup string
	Objective       string
	IgnoreHistory   bool
	filesMu         sync.Mutex
	filesEdited     []string
	readMu          sync.Mutex
	pathModTime     map[string]time.Time
	EnableChallenge *bool
	challengeMu     sync.Mutex
}

const challengeKeep = 30 * time.Minute

type challengeState struct {
	enabled bool
	expire  time.Time
}

var (
	sessionChallengeMu sync.Mutex
	sessionChallenges  = map[string]challengeState{}
)

func (e *Executor) challengeDecision() *bool {
	if e.EnableChallenge != nil {
		e.keepChallenge(*e.EnableChallenge)
		return e.EnableChallenge
	}
	if e.SessionID == "" {
		return nil
	}
	sessionChallengeMu.Lock()
	state, ok := sessionChallenges[e.SessionID]
	if ok && time.Now().After(state.expire) {
		delete(sessionChallenges, e.SessionID)
		ok = false
	}
	sessionChallengeMu.Unlock()
	if !ok {
		return nil
	}
	enabled := state.enabled
	e.EnableChallenge = &enabled
	e.keepChallenge(enabled)
	return e.EnableChallenge
}

func (e *Executor) keepChallenge(enabled bool) {
	if e.SessionID == "" {
		return
	}
	sessionChallengeMu.Lock()
	defer sessionChallengeMu.Unlock()
	sessionChallenges[e.SessionID] = challengeState{enabled: enabled, expire: time.Now().Add(challengeKeep)}
}

func (e *Executor) ChallengeEnabled() bool {
	e.challengeMu.Lock()
	defer e.challengeMu.Unlock()
	decision := e.challengeDecision()
	return decision != nil && *decision
}

func (e *Executor) ResetChallenge() {
	e.challengeMu.Lock()
	defer e.challengeMu.Unlock()
	e.EnableChallenge = nil
	if e.SessionID == "" {
		return
	}
	sessionChallengeMu.Lock()
	defer sessionChallengeMu.Unlock()
	delete(sessionChallenges, e.SessionID)
}

func (e *Executor) DecideChallenge(ask func() (bool, error)) (bool, error) {
	e.challengeMu.Lock()
	defer e.challengeMu.Unlock()
	if decision := e.challengeDecision(); decision != nil {
		return *decision, nil
	}
	enabled, err := ask()
	if err != nil {
		return false, err
	}
	e.EnableChallenge = &enabled
	e.keepChallenge(enabled)
	return enabled, nil
}

func (e *Executor) MarkRead(path string, modTime time.Time) {
	if e == nil || strings.TrimSpace(path) == "" {
		return
	}
	e.readMu.Lock()
	defer e.readMu.Unlock()
	if e.pathModTime == nil {
		e.pathModTime = make(map[string]time.Time)
	}
	e.pathModTime[path] = modTime
}

func (e *Executor) ReadModTime(path string) (time.Time, bool) {
	if e == nil {
		return time.Time{}, false
	}
	e.readMu.Lock()
	defer e.readMu.Unlock()
	modTime, ok := e.pathModTime[path]
	return modTime, ok
}

func (e *Executor) RecordFile(path string) {
	if e == nil || strings.TrimSpace(path) == "" {
		return
	}
	e.filesMu.Lock()
	defer e.filesMu.Unlock()
	if slices.Contains(e.filesEdited, path) {
		return
	}
	e.filesEdited = append(e.filesEdited, path)
}

func (e *Executor) SeedFiles(paths []string) {
	if e == nil {
		return
	}
	for _, one := range paths {
		e.RecordFile(one)
	}
}

func (e *Executor) EditedFiles() []string {
	if e == nil {
		return nil
	}
	e.filesMu.Lock()
	defer e.filesMu.Unlock()
	return slices.Clone(e.filesEdited)
}

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
	Objective       string
	IgnoreHistory   bool
	filesMu         sync.Mutex
	filesEdited     []string
	readMu          sync.Mutex
	pathModTime     map[string]time.Time
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

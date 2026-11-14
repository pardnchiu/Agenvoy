package toolRegister

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
	"github.com/pardnchiu/agenvoy/internal/utils/secret"
)

type Handler func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error)

type GroupHandler func(ctx context.Context, e *tooltypes.Executor, name string, args json.RawMessage) (string, error)

const (
	NoToolTimeout = time.Duration(-1)
)

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     Handler
	AlwaysAllow bool
	Concurrent  bool
	Background  bool
	SystemUse   bool
	Timeout     time.Duration
}

var mu sync.RWMutex

var handlerMap = map[string]Handler{}
var groupHandlerMap = map[string]GroupHandler{}
var defList []go_llm_router.Tool
var builtinNames []string
var readOnlySet = map[string]bool{}
var concurrentSet = map[string]bool{}
var backgroundSet = map[string]bool{}
var systemUseSet = map[string]bool{}
var timeoutMap = map[string]time.Duration{}

func Regist(d Def) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		slog.Warn("toolRegister.Regist: empty name, skipped")
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := handlerMap[d.Name]; exists {
		slog.Warn("toolRegister.Regist: name already registered, overwriting",
			slog.String("name", d.Name))
	}

	raw, _ := json.Marshal(d.Parameters)
	tool := go_llm_router.Tool{
		Type: "function",
		Function: go_llm_router.ToolFunction{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  raw,
		},
	}
	handlerMap[d.Name] = d.Handler
	defList = append(defList, tool)
	builtinNames = append(builtinNames, d.Name)
	if d.AlwaysAllow {
		readOnlySet[d.Name] = true
	}
	if d.Concurrent {
		concurrentSet[d.Name] = true
	}
	if d.Background {
		backgroundSet[d.Name] = true
	}
	if d.SystemUse {
		systemUseSet[d.Name] = true
	}
	if d.Timeout != 0 {
		timeoutMap[d.Name] = d.Timeout
	}
}

func RemoveByPrefix(prefix string) {
	mu.Lock()
	defer mu.Unlock()

	defList = slices.DeleteFunc(defList, func(t go_llm_router.Tool) bool {
		return strings.HasPrefix(t.Function.Name, prefix)
	})
	builtinNames = slices.DeleteFunc(builtinNames, func(n string) bool {
		return strings.HasPrefix(n, prefix)
	})
	for name := range handlerMap {
		if strings.HasPrefix(name, prefix) {
			delete(handlerMap, name)
			delete(readOnlySet, name)
			delete(concurrentSet, name)
			delete(backgroundSet, name)
			delete(systemUseSet, name)
			delete(timeoutMap, name)
		}
	}
}

func GetTimeout(name string) time.Duration {
	mu.RLock()
	defer mu.RUnlock()
	if t, ok := timeoutMap[name]; ok {
		return t
	}
	return configs.DEFAULT_TOOL_TIMEOUT
}

func Exists(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := handlerMap[name]; ok {
		return true
	}
	for prefix := range groupHandlerMap {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func IsReadOnly(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return readOnlySet[name]
}

func MarkAlwaysAllow(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	readOnlySet[name] = true
}

func MarkConcurrent(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	concurrentSet[name] = true
}

func MarkTimeout(name string, timeout time.Duration) {
	name = strings.TrimSpace(name)
	if name == "" || timeout <= 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	timeoutMap[name] = timeout
}

func IsConcurrent(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return concurrentSet[name]
}

func IsBackground(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return backgroundSet[name]
}

func IsSystemUse(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	return systemUseSet[name]
}

func GetTool(name string) *go_llm_router.Tool {
	mu.RLock()
	defer mu.RUnlock()
	for i := range defList {
		if defList[i].Function.Name == name {
			tool := defList[i]
			return &tool
		}
	}
	return nil
}

func BuiltinNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	dst := make([]string, len(builtinNames))
	copy(dst, builtinNames)
	return dst
}

func JSON() []byte {
	mu.RLock()
	defer mu.RUnlock()
	raw, err := json.Marshal(defList)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

func Dispatch(ctx context.Context, e *tooltypes.Executor, name string, args json.RawMessage) (string, error) {
	timeout := GetTimeout(name)
	tctx, cancel := ctx, func() {}
	if timeout > 0 {
		tctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var run func() (string, error)
	mu.RLock()
	if handler, ok := handlerMap[name]; ok {
		run = func() (string, error) { return handler(tctx, e, args) }
	} else {
		for prefix, groupHandler := range groupHandlerMap {
			if strings.HasPrefix(name, prefix) {
				handler := groupHandler
				run = func() (string, error) { return handler(tctx, e, name, args) }
				break
			}
		}
	}
	mu.RUnlock()
	if run == nil {
		return "", utilsmsg.New(utilsmsg.NotFound, "tool", "not exist: %s", name)
	}

	result, err := runWithDeadline(tctx, run, name, timeout)
	return secret.Redact(result), secret.RedactError(err)
}

type dispatchResult struct {
	result string
	err    error
}

func runWithDeadline(tctx context.Context, run func() (string, error), name string, timeout time.Duration) (string, error) {
	done := make(chan dispatchResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("tool handler panicked",
					slog.String("name", name),
					slog.Any("panic", r))
				done <- dispatchResult{err: utilsmsg.New(utilsmsg.Internal, name, "panicked: %v", r)}
			}
		}()
		result, err := run()
		done <- dispatchResult{result: result, err: err}
	}()

	select {
	case out := <-done:
		if tctx.Err() == context.DeadlineExceeded {
			return out.result, utilsmsg.New(utilsmsg.Timeout, name, "timed out after %s", timeout)
		}
		return out.result, out.err
	case <-tctx.Done():
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			slog.Warn("tool handler abandoned after timeout",
				slog.String("name", name),
				slog.String("timeout", timeout.String()))
			return "", utilsmsg.New(utilsmsg.Timeout, name, "timed out after %s and did not honor cancellation", timeout)
		}
		return "", tctx.Err()
	}
}

func RegistGroup(prefix string, handler GroupHandler) {
	mu.Lock()
	defer mu.Unlock()
	groupHandlerMap[prefix] = handler
}

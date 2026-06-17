package toolRegister

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type Handler func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error)

type GroupHandler func(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error)

const DefaultToolTimeout = time.Minute

type Def struct {
	Name          string
	Description   string
	Parameters    map[string]any
	Handler       Handler
	AlwaysAllow   bool
	AlwaysLoad    bool
	Concurrent    bool
	FireAndForget bool
	Timeout       time.Duration
}

var handlerMap = map[string]Handler{}
var groupHandlerMap = map[string]GroupHandler{}
var defList []toolTypes.Tool
var builtinNames []string
var readOnlySet = map[string]bool{}
var alwaysLoadSet = map[string]bool{}
var concurrentSet = map[string]bool{}
var fireAndForgetSet = map[string]bool{}
var timeoutMap = map[string]time.Duration{}

func Regist(d Def) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		slog.Warn("toolRegister.Regist: empty name, skipped")
		return
	}

	if _, exists := handlerMap[d.Name]; exists {
		slog.Warn("toolRegister.Regist: name already registered, overwriting",
			slog.String("name", d.Name))
	}

	raw, _ := json.Marshal(d.Parameters)
	tool := toolTypes.Tool{
		Type: "function",
		Function: toolTypes.ToolFunction{
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
	if d.AlwaysLoad {
		alwaysLoadSet[d.Name] = true
	}
	if d.Concurrent {
		concurrentSet[d.Name] = true
	}
	if d.FireAndForget {
		fireAndForgetSet[d.Name] = true
	}
	if d.Timeout > 0 {
		timeoutMap[d.Name] = d.Timeout
	}
}

func GetTimeout(name string) time.Duration {
	if t, ok := timeoutMap[name]; ok {
		return t
	}
	return DefaultToolTimeout
}

func IsAlwaysLoad(name string) bool {
	return alwaysLoadSet[name]
}

func IsReadOnly(name string) bool {
	return readOnlySet[name]
}

func MarkAlwaysAllow(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	readOnlySet[name] = true
}

func MarkConcurrent(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	concurrentSet[name] = true
}

func MarkTimeout(name string, timeout time.Duration) {
	name = strings.TrimSpace(name)
	if name == "" || timeout <= 0 {
		return
	}
	timeoutMap[name] = timeout
}

func IsConcurrent(name string) bool {
	return concurrentSet[name]
}

func IsFireAndForget(name string) bool {
	return fireAndForgetSet[name]
}

func GetTool(name string) *toolTypes.Tool {
	for i := range defList {
		if defList[i].Function.Name == name {
			return &defList[i]
		}
	}
	return nil
}

func BuiltinNames() []string {
	dst := make([]string, len(builtinNames))
	copy(dst, builtinNames)
	return dst
}

func JSON() []byte {
	raw, err := json.Marshal(defList)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

func Dispatch(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
	timeout := GetTimeout(name)
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	handler, ok := handlerMap[name]
	if ok {
		result, err := handler(tctx, e, args)
		if tctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("tool %q timed out after %s", name, timeout)
		}
		return result, err
	}

	for prefix, handler := range groupHandlerMap {
		if strings.HasPrefix(name, prefix) {
			result, err := handler(tctx, e, name, args)
			if tctx.Err() == context.DeadlineExceeded {
				return result, fmt.Errorf("tool %q timed out after %s", name, timeout)
			}
			return result, err
		}
	}

	return "", fmt.Errorf("not exist: %s", name)
}

func RegistGroup(prefix string, handler GroupHandler) {
	groupHandlerMap[prefix] = handler
}

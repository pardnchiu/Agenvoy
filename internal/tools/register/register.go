package toolRegister

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type Handler func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error)

type GroupHandler func(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error)

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     Handler
	AlwaysAllow    bool
	AlwaysLoad  bool
	Concurrent  bool
}

var handlerMap = map[string]Handler{}
var groupHandlerMap = map[string]GroupHandler{}
var defList []toolTypes.Tool
var readOnlySet = map[string]bool{}
var alwaysLoadSet = map[string]bool{}
var concurrentSet = map[string]bool{}

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

	params, _ := json.Marshal(d.Parameters)
	tool := toolTypes.Tool{
		Type: "function",
		Function: toolTypes.ToolFunction{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  params,
		},
	}
	handlerMap[d.Name] = d.Handler
	defList = append(defList, tool)
	if d.AlwaysAllow {
		readOnlySet[d.Name] = true
	}
	if d.AlwaysLoad {
		alwaysLoadSet[d.Name] = true
	}
	if d.Concurrent {
		concurrentSet[d.Name] = true
	}
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

func IsConcurrent(name string) bool {
	return concurrentSet[name]
}

func JSON() []byte {
	b, err := json.Marshal(defList)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func Dispatch(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
	handler, ok := handlerMap[name]
	if ok {
		return handler(ctx, e, args)
	}

	for prefix, handler := range groupHandlerMap {
		if strings.HasPrefix(name, prefix) {
			return handler(ctx, e, name, args)
		}
	}

	return "", fmt.Errorf("not exist: %s", name)
}

func RegistGroup(prefix string, handler GroupHandler) {
	groupHandlerMap[prefix] = handler
}

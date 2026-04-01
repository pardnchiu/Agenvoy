package toolRegister

import (
	"context"
	"encoding/json"
	"fmt"

	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

type Handler func(ctx context.Context, e *toolTypes.Executor, args json.RawMessage) (string, error)

type GroupHandler func(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error)

type Def struct {
	Name        string
	Description string
	Parameters  map[string]any
	Handler     Handler
	ReadOnly    bool
	AlwaysLoad  bool
}

var handlerMap = map[string]Handler{}
var defList []toolTypes.Tool
var readOnlySet = map[string]bool{}
var alwaysLoadSet = map[string]bool{}

func Regist(d Def) {
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
	if d.ReadOnly {
		readOnlySet[d.Name] = true
	}
	if d.AlwaysLoad {
		alwaysLoadSet[d.Name] = true
	}
}

func IsAlwaysLoad(name string) bool {
	return alwaysLoadSet[name]
}

func IsReadOnly(name string) bool {
	return readOnlySet[name]
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
	if !ok {
		return "", fmt.Errorf("not exist: %s", name)
	}
	return handler(ctx, e, args)
}

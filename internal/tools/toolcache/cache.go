package toolcache

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

type toolHistory struct {
	ToolName  string `json:"tool_name"`
	Args      string `json:"args"`
	Result    string `json:"result"`
	CreatedAt int64  `json:"created_at"`
}

var cacheable = map[string]bool{
	"http_request": true,
}

var (
	globalScope = map[string]bool{
		"fetch_page": true,
	}
	ignoredArg = map[string]bool{
		"force": true,
	}
)

func IsCacheable(name, args string) bool {
	if !cacheable[name] {
		return false
	}
	if name != "http_request" {
		return true
	}
	return IsIdempotentRequest(args)
}

func IsIdempotentRequest(args string) bool {
	var params struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return false
	}
	method := strings.ToUpper(strings.TrimSpace(params.Method))
	return method == "" || method == "GET"
}

func keyPrefix(sessionID, toolName string) string {
	if globalScope[toolName] {
		return "tc:global:"
	}
	return "tc:" + sessionID + ":"
}

func schemaDefault(toolName string) map[string]any {
	tool := toolregister.GetTool(toolName)
	if tool == nil {
		return nil
	}
	var schema struct {
		Properties map[string]struct {
			Default any `json:"default"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
		return nil
	}
	dic := make(map[string]any, len(schema.Properties))
	for name, prop := range schema.Properties {
		if prop.Default != nil {
			dic[name] = prop.Default
		}
	}
	return dic
}

func canonical(toolName, args string) string {
	var dic map[string]any
	if err := json.Unmarshal([]byte(args), &dic); err != nil {
		return args
	}
	for name := range ignoredArg {
		delete(dic, name)
	}
	for name, value := range schemaDefault(toolName) {
		if _, ok := dic[name]; ok || ignoredArg[name] {
			continue
		}
		dic[name] = value
	}
	raw, err := json.Marshal(dic)
	if err != nil {
		return args
	}
	return string(raw)
}

func Store(sessionID, callID, toolName, args, result string) {
	raw, err := json.Marshal(toolHistory{
		ToolName:  toolName,
		Args:      canonical(toolName, args),
		Result:    result,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		return
	}
	db := torii.DB(torii.DBToolCache)
	if err := db.Set(context.Background(), keyPrefix(sessionID, toolName)+callID, string(raw), torii.TTL(configs.TTL_TOOL_CACHE_SEC)); err != nil {
		slog.Debug("toolcache Store",
			slog.String("session", sessionID),
			slog.String("error", err.Error()))
	}
}

func FindRecent(sessionID, toolName, args string) (string, bool) {
	db := torii.DB(torii.DBToolCache)
	want := canonical(toolName, args)
	var best toolHistory
	found := false
	for _, entry := range db.Scan(context.Background(), keyPrefix(sessionID, toolName)+"*", torii.ScanOption{}) {
		var e toolHistory
		if err := json.Unmarshal([]byte(entry.Value()), &e); err != nil {
			continue
		}
		if e.ToolName != toolName || e.Args != want {
			continue
		}
		if !found || e.CreatedAt > best.CreatedAt {
			best = e
			found = true
		}
	}
	return best.Result, found
}

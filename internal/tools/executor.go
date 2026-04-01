package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	scriptAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/script"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func NewExecutor(workPath, sessionID string) (*toolTypes.Executor, error) {
	var tools []toolTypes.Tool
	if err := json.Unmarshal(toolRegister.JSON(), &tools); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	var commands []string
	if err := json.Unmarshal(configs.WhiteList, &commands); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	allowedCommand := make(map[string]bool, len(commands))
	for _, cmd := range commands {
		allowedCommand[cmd] = true
	}

	apiToolbox := apiAdapter.New()
	apiToolbox.LoadFS(extensions.APIs, "apis")

	for _, dir := range []string{
		filesystem.APIToolsDir,
		filesystem.WorkAPIToolsDir,
	} {
		apiToolbox.Load(dir)
	}

	for _, tool := range apiToolbox.GetTools() {
		data, err := json.Marshal(tool)
		if err != nil {
			continue
		}
		var t toolTypes.Tool
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		tools = append(tools, t)
	}

	scriptToolbox := scriptAdapter.New()
	for _, dir := range []string{
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
	} {
		scriptToolbox.Scan(dir)
	}

	for _, tool := range scriptToolbox.GetTools() {
		data, err := json.Marshal(tool)
		if err != nil {
			continue
		}
		var t toolTypes.Tool
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		tools = append(tools, t)
	}

	// * order fixed, for cache hit
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Function.Name < tools[j].Function.Name
	})

	// * use claude code idea, use one tool to search and insert
	stubParams := json.RawMessage(`{"type":"object","properties":{}}`)
	stubTools := make(map[string]bool, len(tools))
	initial := make([]toolTypes.Tool, 0, len(tools))
	for _, t := range tools {
		if toolRegister.IsAlwaysLoad(t.Function.Name) {
			initial = append(initial, t)
		} else {
			stubTools[t.Function.Name] = true
			initial = append(initial, toolTypes.Tool{
				Type: t.Type,
				Function: toolTypes.ToolFunction{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  stubParams,
				},
			})
		}
	}

	return &toolTypes.Executor{
		WorkDir:        workPath,
		SessionID:      sessionID,
		AllowedCommand: allowedCommand,
		Tools:          initial,
		AllTools:       tools,
		StubTools:      stubTools,
		APIToolbox:     apiToolbox,
		ScriptToolbox:  scriptToolbox,
	}, nil
}

func normalizeArgs(args json.RawMessage) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return args
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			var unquoted string
			if err := json.Unmarshal([]byte(`"`+s+`"`), &unquoted); err == nil {
				m[k] = unquoted
			}
		}
	}
	if out, err := json.Marshal(m); err == nil {
		return out
	}
	return args
}

func Execute(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
	args = normalizeArgs(args)

	if e.StubTools[name] {
		activateArgs, _ := json.Marshal(map[string]any{"query": "select:" + name})
		_, _ = toolRegister.Dispatch(ctx, e, "search_tools", activateArgs)
		delete(e.StubTools, name)
		return `{"status":"schema_activated","message":"Tool schema has been loaded. Please call this tool again with the correct parameters based on the updated schema."}`, nil
	}

	if strings.HasPrefix(name, "api_") && e.APIToolbox != nil && e.APIToolbox.IsExist(name) {
		var params map[string]any
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return e.APIToolbox.Execute(name, params)
	}

	if strings.HasPrefix(name, "script_") && e.ScriptToolbox != nil && e.ScriptToolbox.IsExist(name) {
		return e.ScriptToolbox.Execute(ctx, name, args, e.WorkDir)
	}

	return toolRegister.Dispatch(ctx, e, name, args)
}

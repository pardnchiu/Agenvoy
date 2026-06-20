package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	scriptAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/script"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func NewExecutor(workPath, sessionID string, scanner *runtime.SkillScanner) (*toolTypes.Executor, error) {
	var tools []toolTypes.Tool
	if err := json.Unmarshal(toolRegister.JSON(), &tools); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	allowedCommand := make(map[string]bool, len(filesystem.WhiteList))
	for _, cmd := range filesystem.WhiteList {
		allowedCommand[cmd] = true
	}

	apiToolbox := apiAdapter.New("api_")
	apiToolbox.LoadFS(extensions.APIs, "apis")

	for _, dir := range []string{
		filesystem.LegacyAPIToolsDir,
		filesystem.LegacyWorkAPIToolsDir,
		filesystem.APIToolsDir,
		filesystem.WorkAPIToolsDir,
	} {
		apiToolbox.Load(dir)
	}

	extAPIToolbox := apiAdapter.New("ext_")
	extAPIToolbox.LoadDirs(filesystem.ExtensionAPIToolsDir)

	for _, tb := range []*apiAdapter.Translator{apiToolbox, extAPIToolbox} {
		for _, tool := range tb.GetTools() {
			raw, err := json.Marshal(tool)
			if err != nil {
				continue
			}
			var t toolTypes.Tool
			if err := json.Unmarshal(raw, &t); err != nil {
				continue
			}
			tools = append(tools, t)
		}
		for _, name := range tb.AlwaysAllowNames() {
			toolRegister.MarkAlwaysAllow(name)
		}
		for _, name := range tb.ConcurrentNames() {
			toolRegister.MarkConcurrent(name)
		}
	}

	scriptToolbox := scriptAdapter.New("script_")
	for _, dir := range []string{
		filesystem.SystemToolsDir,
		filesystem.LegacyScriptToolsDir,
		filesystem.LegacyWorkScriptToolsDir,
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
	} {
		scriptToolbox.Scan(dir)
	}

	extScriptToolbox := scriptAdapter.New("ext_")
	extScriptToolbox.Scan(filesystem.ExtensionScriptToolsDir)

	for _, tb := range []*scriptAdapter.Translator{scriptToolbox, extScriptToolbox} {
		for _, tool := range tb.GetTools() {
			raw, err := json.Marshal(tool)
			if err != nil {
				continue
			}
			var t toolTypes.Tool
			if err := json.Unmarshal(raw, &t); err != nil {
				continue
			}
			tools = append(tools, t)
		}
		for _, name := range tb.AlwaysAllowNames() {
			toolRegister.MarkAlwaysAllow(name)
		}
		for _, name := range tb.ConcurrentNames() {
			toolRegister.MarkConcurrent(name)
		}
		for name, timeoutSec := range tb.Timeouts() {
			toolRegister.MarkTimeout(name, time.Duration(timeoutSec)*time.Second)
		}
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
		WorkDir:          workPath,
		SessionID:        sessionID,
		AllowedCommand:   allowedCommand,
		Tools:            initial,
		AllTools:         tools,
		StubTools:        stubTools,
		APIToolbox:       apiToolbox,
		ScriptToolbox:    scriptToolbox,
		ExtAPIToolbox:    extAPIToolbox,
		ExtScriptToolbox: extScriptToolbox,
		SkillScanner:     scanner,
	}, nil
}

func normalizeArgs(args json.RawMessage) json.RawMessage {
	var dic map[string]any
	if err := json.Unmarshal(args, &dic); err != nil {
		return args
	}
	for k, v := range dic {
		if s, ok := v.(string); ok {
			var unquoted string
			if err := json.Unmarshal([]byte(`"`+s+`"`), &unquoted); err == nil {
				dic[k] = unquoted
			}
		}
	}
	if raw, err := json.Marshal(dic); err == nil {
		return raw
	}
	return args
}

func Execute(ctx context.Context, e *toolTypes.Executor, name string, args json.RawMessage) (string, error) {
	args = normalizeArgs(args)

	if e.StubTools[name] {
		activateArgs, _ := json.Marshal(map[string]any{"query": "select:" + name})
		if _, err := toolRegister.Dispatch(ctx, e, "search_tools", activateArgs); err != nil {
			slog.Warn("stub tool activation failed",
				slog.String("name", name),
				slog.String("error", err.Error()))
		}
		delete(e.StubTools, name)
	}

	return toolRegister.Dispatch(ctx, e, name, args)
}

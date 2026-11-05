package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"sort"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	scriptadapter "github.com/pardnchiu/agenvoy/internal/runtime/toolAdapter/script"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func NewExecutor(workPath, sessionID string, scanner *runtime.SkillScanner) (*tooltypes.Executor, error) {
	var tools []go_llm_router.Tool
	if err := json.Unmarshal(toolregister.JSON(), &tools); err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "json.Unmarshal", "", err)
	}

	warnLegacyAPITools()

	scriptToolbox := scriptadapter.New("script_")
	for _, dir := range []string{
		filesystem.SystemToolsDir,
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
	} {
		scriptToolbox.Scan(dir)
	}

	extScriptToolbox := scriptadapter.New("ext_")
	extScriptToolbox.Scan(filesystem.ExtensionScriptToolsDir)

	for _, tb := range []*scriptadapter.Translator{scriptToolbox, extScriptToolbox} {
		for _, tool := range tb.GetTools() {
			raw, err := json.Marshal(tool)
			if err != nil {
				continue
			}
			var t go_llm_router.Tool
			if err := json.Unmarshal(raw, &t); err != nil {
				continue
			}
			tools = append(tools, t)
		}
		for _, name := range tb.AlwaysAllowNames() {
			toolregister.MarkAlwaysAllow(name)
		}
		for _, name := range tb.ConcurrentNames() {
			toolregister.MarkConcurrent(name)
		}
		for name, timeoutSec := range tb.Timeouts() {
			toolregister.MarkTimeout(name, time.Duration(timeoutSec)*time.Second)
		}
	}

	// * order fixed, for cache hit
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Function.Name < tools[j].Function.Name
	})

	initial := make([]go_llm_router.Tool, 0, len(tools))
	unmarkedTools := make(map[string]bool)
	for _, tool := range tools {
		name := tool.Function.Name
		if slices.Contains(configs.SYSTEM_DEFAULT_TOOLS, name) {
			initial = append(initial, tool)
			continue
		}
		unmarkedTools[name] = true
	}

	return &tooltypes.Executor{
		WorkDir:          workPath,
		SessionID:        sessionID,
		Tools:            initial,
		AllTools:         tools,
		UnmarkedTools:    unmarkedTools,
		ScriptToolbox:    scriptToolbox,
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

func Execute(ctx context.Context, e *tooltypes.Executor, name string, args json.RawMessage) (string, error) {
	args = normalizeArgs(args)

	if e.UnmarkedTools[name] {
		activateArgs, _ := json.Marshal(map[string]any{"mode": "search", "names": []string{name}})
		if _, err := toolregister.Dispatch(ctx, e, "find_tools", activateArgs); err != nil {
			slog.Warn("tool schema marking failed",
				slog.String("name", name),
				slog.String("error", err.Error()))
		}
		delete(e.UnmarkedTools, name)
	}

	return toolregister.Dispatch(ctx, e, name, args)
}

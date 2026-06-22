package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	scriptAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/script"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
)

type builtinHandler func(ctx context.Context, args json.RawMessage) (string, error)

type toolbox struct {
	scriptBox *scriptAdapter.Translator
	apiBox    *apiAdapter.Adapter
	extScript *scriptAdapter.Translator
	extAPI    *apiAdapter.Adapter
	tools     []mcpTool
	builtin   map[string]builtinHandler
}

func scanTools() *toolbox {
	box := &toolbox{builtin: make(map[string]builtinHandler)}

	box.scriptBox = scriptAdapter.New("script_")
	for _, dir := range []string{
		filesystem.SystemToolsDir,
		filesystem.LegacyScriptToolsDir,
		filesystem.LegacyWorkScriptToolsDir,
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
	} {
		if err := box.scriptBox.Scan(dir); err != nil {
			slog.Warn("scriptBox.Scan",
				slog.String("dir", dir),
				slog.String("error", err.Error()))
		}
	}

	box.extScript = scriptAdapter.New("ext_")
	if err := box.extScript.Scan(filesystem.ExtensionScriptToolsDir); err != nil {
		slog.Warn("extScript.Scan",
			slog.String("error", err.Error()))
	}

	box.apiBox = apiAdapter.New("api_")
	_ = box.apiBox.Builtin(extensions.APIs, "apis")
	for _, dir := range []string{
		filesystem.LegacyAPIToolsDir,
		filesystem.LegacyWorkAPIToolsDir,
		filesystem.APIToolsDir,
		filesystem.WorkAPIToolsDir,
	} {
		_ = box.apiBox.Load(dir)
	}

	box.extAPI = apiAdapter.New("ext_")
	_ = box.extAPI.Load(filesystem.ExtensionAPIToolsDir)

	var tools []mcpTool
	tools = append(tools, convertTools(box.scriptBox.GetTools())...)
	tools = append(tools, convertTools(box.extScript.GetTools())...)
	tools = append(tools, convertTools(box.apiBox.GetTools())...)
	tools = append(tools, convertTools(box.extAPI.GetTools())...)

	if tools == nil {
		tools = []mcpTool{}
	}
	box.tools = tools

	addGuide(box)
	addBuiltins(box)

	return box
}

func (b *toolbox) dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if handler, ok := b.builtin[name]; ok {
		return handler(ctx, args)
	}

	workDir, _ := os.Getwd()

	switch {
	case b.scriptBox.IsExist(name):
		return b.scriptBox.Execute(ctx, name, args, workDir)
	case b.extScript.IsExist(name):
		return b.extScript.Execute(ctx, name, args, workDir)
	case b.apiBox.IsExist(name):
		var dic map[string]any
		if err := json.Unmarshal(args, &dic); err != nil || dic == nil {
			dic = map[string]any{}
		}
		return b.apiBox.Execute(ctx, name, dic)
	case b.extAPI.IsExist(name):
		var dic map[string]any
		if err := json.Unmarshal(args, &dic); err != nil || dic == nil {
			dic = map[string]any{}
		}
		return b.extAPI.Execute(ctx, name, dic)
	default:
		return "", fmt.Errorf("tool not found: %s", name)
	}
}

func addGuide(box *toolbox) {
	box.tools = append(box.tools, mcpTool{
		Name: "tool_generate_guide",
		Description: `
You MUST call this tool before refusing any user request.
If the user asks for something and no other tool here can handle it (weather, prices, data lookup, conversion, scraping, analysis, etc.),
call this to get instructions for building a new tool on the fly.
Do NOT say 'I cannot' or 'no tool available' — always build one instead.
Covers both script tools (Python, multi-step) and API tools (single JSON, declarative REST).
After reading the guide: write_tool -> test_tool (script only) -> call the new tool to answer.`,
		InputSchema: emptySchema(),
	})

	box.builtin["tool_generate_guide"] = func(_ context.Context, _ json.RawMessage) (string, error) {
		return configs.ToolGuide, nil
	}

	box.builtin["script_tool_generate_guide"] = func(_ context.Context, _ json.RawMessage) (string, error) {
		return configs.ToolGuide, nil
	}
}

var bridgedTools = []string{
	"write_tool",
	"test_tool",
	"patch_tool",
	"remove_tool",
	"store_secret",
}

func addBuiltins(box *toolbox) {
	for _, name := range bridgedTools {
		tool := toolRegister.GetTool(name)
		if tool == nil {
			continue
		}
		box.tools = append(box.tools, mcpTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: json.RawMessage(tool.Function.Parameters),
		})
		n := name
		box.builtin[n] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return toolRegister.Dispatch(ctx, nil, n, args)
		}
	}

	box.tools = append(box.tools, mcpTool{
		Name:        "list_tools",
		Description: "List all tools exposed by this MCP server with name and description.",
		InputSchema: emptySchema(),
	})
	box.builtin["list_tools"] = func(_ context.Context, _ json.RawMessage) (string, error) {
		type entry struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		list := make([]entry, 0, len(box.tools))
		for _, t := range box.tools {
			list = append(list, entry{Name: t.Name, Description: t.Description})
		}
		raw, _ := json.Marshal(list)
		return string(raw), nil
	}
}

func convertTools(openAI []map[string]any) []mcpTool {
	tools := make([]mcpTool, 0, len(openAI))
	for _, t := range openAI {
		fn, ok := t["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)

		var schema json.RawMessage
		if params, ok := fn["parameters"]; ok {
			schema, _ = json.Marshal(params)
		}
		if len(schema) == 0 {
			schema = emptySchema()
		}

		tools = append(tools, mcpTool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools
}

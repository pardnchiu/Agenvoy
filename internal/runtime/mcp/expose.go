package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	scriptadapter "github.com/pardnchiu/agenvoy/internal/runtime/toolAdapter/script"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type builtinHandler func(ctx context.Context, args json.RawMessage) (string, error)

type toolbox struct {
	scriptBox *scriptadapter.Translator
	extScript *scriptadapter.Translator
	tools     []Tool
	builtin   map[string]builtinHandler
}

func scanTools() *toolbox {
	box := &toolbox{builtin: make(map[string]builtinHandler)}

	box.scriptBox = scriptadapter.New("script_")
	for _, dir := range []string{
		filesystem.SystemToolsDir,
		filesystem.ScriptToolsDir,
		filesystem.WorkScriptToolsDir,
	} {
		if err := box.scriptBox.Scan(dir); err != nil {
			slog.Warn("scriptBox.Scan",
				slog.String("dir", dir),
				slog.String("error", err.Error()))
		}
	}

	box.extScript = scriptadapter.New("ext_")
	if err := box.extScript.Scan(filesystem.ExtensionScriptToolsDir); err != nil {
		slog.Warn("extScript.Scan",
			slog.String("error", err.Error()))
	}

	var tools []Tool
	tools = append(tools, convertTools(box.scriptBox.GetTools())...)
	tools = append(tools, convertTools(box.extScript.GetTools())...)

	if tools == nil {
		tools = []Tool{}
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
	default:
		return "", utilsmsg.New(utilsmsg.NotFound, "tool", "%s", name)
	}
}

func addGuide(box *toolbox) {
	box.tools = append(box.tools, Tool{
		Name: "tool_generate_guide",
		Description: `You MUST call this tool before refusing any user request: no other tool here covers it (weather, prices, data lookup, conversion, scraping, analysis, etc.) → call this for instructions to build one on the fly, never say 'I cannot' or 'no tool available'.
Starts with public_api_list: a fitting API is read and called directly; only without one does a Python script tool get built (also for a user-supplied API that needs a key).
After reading the guide: edit_tool(mode=write) -> test_tool -> call the new tool to answer.`,
		InputSchema: emptySchema(),
	})

	box.builtin["tool_generate_guide"] = func(_ context.Context, _ json.RawMessage) (string, error) {
		return configs.GuideToolGenerate, nil
	}

	box.builtin["script_tool_generate_guide"] = func(_ context.Context, _ json.RawMessage) (string, error) {
		return configs.GuideToolGenerate, nil
	}
}

var bridgedTools = []string{
	"public_api_list",
	"edit_tool",
	"test_tool",
	"store_secret",
}

func addBuiltins(box *toolbox) {
	for _, name := range bridgedTools {
		tool := toolregister.GetTool(name)
		if tool == nil {
			continue
		}
		box.tools = append(box.tools, Tool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: json.RawMessage(tool.Function.Parameters),
		})
		n := name
		box.builtin[n] = func(ctx context.Context, args json.RawMessage) (string, error) {
			return toolregister.Dispatch(ctx, nil, n, args)
		}
	}

	box.tools = append(box.tools, Tool{
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

func convertTools(openAI []map[string]any) []Tool {
	tools := make([]Tool, 0, len(openAI))
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

		tools = append(tools, Tool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools
}

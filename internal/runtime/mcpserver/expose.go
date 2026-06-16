package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	apiAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/api"
	scriptAdapter "github.com/pardnchiu/agenvoy/internal/toolAdapter/script"
)

type toolbox struct {
	scriptBox *scriptAdapter.Translator
	apiBox    *apiAdapter.Translator
	extScript *scriptAdapter.Translator
	extAPI    *apiAdapter.Translator
	tools     []mcpTool
}

func scanTools() *toolbox {
	box := &toolbox{}

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
	_ = box.apiBox.LoadFS(extensions.APIs, "apis")
	for _, dir := range []string{
		filesystem.LegacyAPIToolsDir,
		filesystem.LegacyWorkAPIToolsDir,
		filesystem.APIToolsDir,
		filesystem.WorkAPIToolsDir,
	} {
		_ = box.apiBox.Load(dir)
	}

	box.extAPI = apiAdapter.New("ext_")
	_ = box.extAPI.LoadDirs(filesystem.ExtensionAPIToolsDir)

	var tools []mcpTool
	tools = append(tools, convertTools(box.scriptBox.GetTools())...)
	tools = append(tools, convertTools(box.extScript.GetTools())...)
	tools = append(tools, convertTools(box.apiBox.GetTools())...)
	tools = append(tools, convertTools(box.extAPI.GetTools())...)

	if tools == nil {
		tools = []mcpTool{}
	}
	box.tools = tools
	return box
}

func (b *toolbox) dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
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
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		tools = append(tools, mcpTool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools
}

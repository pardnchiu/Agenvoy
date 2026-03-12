package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/extensions"
	"github.com/pardnchiu/agenvoy/internal/tools/apiAdapter"
	"github.com/pardnchiu/agenvoy/internal/tools/apis"
	"github.com/pardnchiu/agenvoy/internal/tools/apis/searchWeb"
	"github.com/pardnchiu/agenvoy/internal/tools/browser"
	"github.com/pardnchiu/agenvoy/internal/tools/calculator"
	"github.com/pardnchiu/agenvoy/internal/tools/cron"
	"github.com/pardnchiu/agenvoy/internal/tools/file"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

//go:embed embed/tools.json
var toolsMap []byte

func NewExecutor(workPath, sessionID string) (*toolTypes.Executor, error) {
	var tools []toolTypes.Tool
	if err := json.Unmarshal(toolsMap, &tools); err != nil {
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

	if configDir, err := utils.GetConfigDir("apis"); err == nil {
		apiToolbox.Load(configDir.Home)
		apiToolbox.Load(configDir.Work)
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

	return &toolTypes.Executor{
		WorkPath:       workPath,
		SessionID:      sessionID,
		AllowedCommand: allowedCommand,
		Exclude:        file.ListExcludes(workPath),
		Tools:          tools,
		APIToolbox:     apiToolbox,
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
	// * get all api tools
	if strings.HasPrefix(name, "api_") && e.APIToolbox != nil && e.APIToolbox.IsExist(name) {
		var params map[string]any
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return e.APIToolbox.Execute(name, params)
	}

	switch name {
	case "read_file", "list_files", "glob_files", "search_content", "search_history", "write_file", "patch_edit", "get_tool_error", "remember_error", "search_errors":
		return file.Routes(e, name, args)

	case "send_http_request", "fetch_google_rss":
		return apis.Routes(e, name, args)

	case "run_command":
		var params struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return runCommand(ctx, e, params.Command)

	case "fetch_page":
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("failed to unmarshal json (%s): %w", name, err)
		}
		return browser.Load(params.URL)

	case "download_page":
		var params struct {
			Href   string `json:"href"`
			SaveTo string `json:"save_to"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("failed to unmarshal json (%s): %w", name, err)
		}
		return browser.Download(params.Href, params.SaveTo)

	case "search_web":
		var params struct {
			Query string `json:"query"`
			Range string `json:"range"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("failed to unmarshal json (%s): %w", name, err)
		}
		return searchWeb.Search(ctx, params.Query, searchWeb.TimeRange(params.Range))

	case "write_scheduler_script":
		var params struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return cron.WriteScript(params.Name, params.Content)

	case "add_onetime_task":
		var params struct {
			At     string `json:"at"`
			Script string `json:"script"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return cron.AddOneTimeTask(params.At, params.Script)

	case "calculate":
		var params struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("json.Unmarshal: %w", err)
		}
		return calculator.Calc(params.Expression)

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

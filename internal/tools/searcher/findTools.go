package toolSearcher

import (
	"context"
	"encoding/json"
	"strings"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const (
	systemDefaultMarker = "[system-default]"
	maxSelectNames      = 16
)

type Tool struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Parameters    json.RawMessage `json:"parameters,omitempty"`
	SystemDefault bool            `json:"system_default,omitempty"`
}

func registFindTools() {
	toolregister.Regist(toolregister.Def{
		Name:        "find_tools",
		SystemUse:   true,
		AlwaysAllow: true,
		Concurrent:  true,
		Description: `Returns full parameter schemas for the tools you name or describe (search), or every tool's name and one-line description (list).
Use before the first run_tool call to a name, and for 有哪些工具 / 找工具 / 有沒有可以...的工具 / search_tools / list_tools; a capability that looks missing is checked here before anything is built.
Building a tool → edit_tool; testing a script tool → test_tool.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"search", "list"},
					"description": "search returns schemas for names or query; list returns names and descriptions only. Omitted: names or query → search, otherwise list.",
				},
				"names": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"maxItems":    maxSelectNames,
					"description": "mode=search: exact names from the Tools list, at most 16 per call. When several fit one intent, prefer mcp__* over script_* over [system-default].",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "mode=search: keywords that must all match; only when no listed name fits.",
				},
			},
		},
		Handler: func(_ context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode  string   `json:"mode"`
				Names []string `json:"names"`
				Query string   `json:"query"`
			}
			if len(args) > 0 {
				if err := json.Unmarshal(args, &params); err != nil {
					return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
				}
			}

			params.Mode = strings.TrimSpace(params.Mode)
			params.Query = strings.TrimSpace(params.Query)
			if params.Mode == "" {
				params.Mode = "list"
				if len(params.Names) > 0 || params.Query != "" {
					params.Mode = "search"
				}
			}

			switch params.Mode {
			case "search":
				if len(params.Names) > maxSelectNames {
					return "", utilsmsg.New(utilsmsg.Limit, "names", "%d names; at most %d per call, split the rest into another call", len(params.Names), maxSelectNames)
				}
				if len(params.Names) > 0 {
					return searchTools(e, "select:"+strings.Join(params.Names, ","))
				}
				if params.Query == "" {
					return "", utilsmsg.New(utilsmsg.Invalid, "names", "names or query required when mode=search")
				}
				return searchTools(e, params.Query)
			case "list":
				return listTools(e)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown mode %q; available: search, list", params.Mode)
		},
	})
}

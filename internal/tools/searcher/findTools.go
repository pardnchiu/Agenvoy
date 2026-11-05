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
		Description: `The tool registry: what exists (list), and pulling a tool's schema in so it can be called (search).
Use for 有哪些工具 / 找工具 / 有沒有可以...的工具, and for search_tools / list_tools.
A capability that seems missing comes from here before anything is built. Building one → edit_tool; running a script tool → test_tool.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"search", "list"},
					"description": "search: match the registry and return the matched tools' full parameter schemas. list: names and one-line descriptions only. Omitted: names or query → search, otherwise list.",
					"default":     "search",
				},
				"names": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"maxItems":    maxSelectNames,
					"description": "mode=search: exact names from the Tools list, at most 16 per call; more → another call. Prefer unmarked tools (mcp__* > script_*) over [system-default] for the same intent.",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "mode=search: keywords, all of which must match; only when no listed name fits.",
				},
				"mcp": map[string]any{
					"type":        "boolean",
					"description": "mode=list: only MCP-exposed tools — builtin + script_/ext_ prefixed.",
					"default":     false,
				},
				"system": map[string]any{
					"type":        "boolean",
					"description": "mode=list: also list the system tools used for internal bookkeeping.",
					"default":     false,
				},
			},
		},
		Handler: func(_ context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Mode   string   `json:"mode"`
				Names  []string `json:"names"`
				Query  string   `json:"query"`
				MCP    bool     `json:"mcp"`
				System bool     `json:"system"`
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
				return listTools(e, params.MCP, params.System)
			}
			return "", utilsmsg.New(utilsmsg.Invalid, "mode", "unknown mode %q; available: search, list", params.Mode)
		},
	})
}

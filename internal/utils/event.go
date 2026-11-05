package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	"github.com/pardnchiu/go-llm-router/core/claudeCode"
	"github.com/pardnchiu/go-llm-router/core/copilot"
	"github.com/pardnchiu/go-llm-router/core/deepseek"
	grokoauth "github.com/pardnchiu/go-llm-router/core/grokOauth"
	openrouter "github.com/pardnchiu/go-llm-router/core/openRouter"
	openaicodex "github.com/pardnchiu/go-llm-router/core/openaiCodex"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	agentkeychain "github.com/pardnchiu/agenvoy/internal/agents/keychain"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var eventLabel = map[string]string{
	"read_files":            "File",
	"run_command":           "Run",
	"background_command":    "Background",
	"background_subagents":  "Background Subagents",
	"open_file":             "Open",
	"download_file":         "Download",
	"search_web":            "Search",
	"fetch_page":            "Fetch",
	"http_request":          "Send",
	"calculate":             "Calc",
	"test_tool":             "Test",
	"send_to_chatbot":       "Push",
	"list_chatbot":          "Chatbots",
	"mcp__kura__search_rag": "RAG",
	"mcp__kura__list_rag":   "RAG",
	"reasoning_guide":       "Guide",
	"write_result":          "Write",
	"find_tools":            "Tools",
	"write_todo":            "Plan",
}

func parseToolArgs(raw string) map[string]any {
	var argMap map[string]any
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &argMap)
	}
	return argMap
}

func hideToolEvent(name, mode string) bool {
	return name == ""
}

func HideToolEvent(name, raw string) bool {
	return hideToolEvent(name, eventMode(name, parseToolArgs(raw)))
}

var primaryArgKeys = []string{
	"q", "query", "keyword", "input", "text", "prompt",
	"symbol", "ticker", "db", "topic", "name", "url", "link", "path", "id",
}

var eventModeLabel = map[string]map[string]string{
	"find_files": {"list": "Files", "glob": "Glob Files", "search": "Search Files"},
	"edit_file":  {"write": "Write", "patch": "Edit", "remove": "Remove", "restore": "Restore"},
	"schedules":  {"list": "Schedules", "patch": "Edit", "remove": "Remove", "write": "Write"},
	"subagents":  {"invoke": "Subagents", "list": "Subagents"},
	"edit_skill": {"write": "Write Skill", "patch": "Edit Skill", "remove": "Remove Skill"},
	"edit_tool":  {"write": "Write Tool", "patch": "Edit Tool", "remove": "Remove Tool"},
}

func FormatToolEvent(name, raw string) string {
	argMap := parseToolArgs(raw)
	mode := eventMode(name, argMap)
	if hideToolEvent(name, mode) {
		return ""
	}

	arg := func(keys ...string) string {
		for _, key := range keys {
			if val, ok := argMap[key]; ok {
				if str, ok := val.(string); ok && strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str)
				}
			}
		}
		return ""
	}

	label := eventLabel[name]
	if dic, ok := eventModeLabel[name]; ok {
		label = dic[mode]
	}
	if label == "" {
		label = shortToolName(name)
	}

	return label + "(" + eventArgs(name, mode, raw, argMap, arg) + ")"
}

func eventArgs(name, mode, raw string, argMap map[string]any, arg func(...string) string) string {
	switch name {
	case "find_files":
		return joinQueries(argMap)

	case "read_files":
		return joinPaths(argMap)

	case "edit_file":
		if path := arg("path"); path != "" {
			return path
		}
		if v, ok := argMap["version"].(float64); ok && v > 0 {
			return fmt.Sprintf("version %d", int64(v))
		}
		return arg("task_id")

	case "schedules":
		return arg("skill_name")

	case "subagents", "background_subagents":
		if name == "subagents" && mode != "invoke" {
			return ""
		}
		if target := arg("self_id"); target != "" {
			return target
		}
		return arg("model")

	case "run_command", "background_command":
		return joinArgv(raw)

	case "open_file":
		return arg("path")

	case "download_file":
		return arg("url", "link")

	case "fetch_page":
		return arg("link", "url")

	case "http_request":
		url := arg("url")
		if method := arg("method"); method != "" && url != "" {
			return strings.ToUpper(method) + " " + url
		}
		return url

	case "search_web":
		query := arg("query", "keyword")
		if timeRange := arg("time_range", "time"); query != "" && timeRange != "" {
			return query + " [" + timeRange + "]"
		}
		return query

	case "calculate":
		return joinStrings(argMap["expressions"])

	case "run_skill":
		return arg("skill", "name")

	case "edit_skill":
		return arg("path", "name")

	case "edit_tool", "test_tool":
		return arg("name")

	case "reasoning_guide":
		return joinStrings(argMap["topics"])

	case "write_result":
		format := arg("format")
		if format == "" {
			format = "md"
		}
		return filepath.Join(filesystem.OutputDir(), "report-*."+format)

	case "find_tools":
		if names := joinStrings(argMap["names"]); names != "" {
			return names
		}
		return arg("query")

	case "write_todo":
		return todoProgress(argMap["todos"])
	}

	if len(argMap) == 0 {
		return ""
	}
	if value := arg(primaryArgKeys...); value != "" {
		return value
	}
	return strings.Join(strings.Fields(raw), " ")
}

func eventMode(name string, argMap map[string]any) string {
	if mode, ok := argMap["mode"].(string); ok && strings.TrimSpace(mode) != "" {
		return strings.TrimSpace(mode)
	}

	switch name {
	case "find_files":
		mode := "list"
		for _, one := range asList(argMap["queries"]) {
			pattern, _ := one["pattern"].(string)
			if strings.TrimSpace(pattern) == "" {
				continue
			}
			if filePattern, _ := one["file_pattern"].(string); strings.TrimSpace(filePattern) != "" {
				return "search"
			}
			mode = "glob"
		}
		return mode

	case "edit_file":
		if targets, ok := argMap["targets"].([]any); ok && len(targets) > 0 {
			return "patch"
		}
		if content, _ := argMap["content"].(string); content != "" {
			return "write"
		}

	case "edit_skill", "edit_tool":
		if content, _ := argMap["content"].(string); content != "" {
			return "write"
		}
		if old, _ := argMap["old_string"].(string); old != "" {
			return "patch"
		}

	case "subagents":
		if task, _ := argMap["task"].(string); strings.TrimSpace(task) != "" {
			return "invoke"
		}
		return "list"

	case "schedules":
		return "list"
	}
	return ""
}

func shortToolName(name string) string {
	if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
		if _, tool, found := strings.Cut(rest, "__"); found && tool != "" {
			return tool
		}
		return rest
	}
	if IsPlugTool(name) {
		return PlugToolBaseName(name)
	}
	return name
}

func asList(value any) []map[string]any {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, one := range list {
		if dic, ok := one.(map[string]any); ok {
			out = append(out, dic)
		}
	}
	return out
}

func joinQueries(argMap map[string]any) string {
	labels := make([]string, 0, 4)
	for _, one := range asList(argMap["queries"]) {
		dir, _ := one["dir"].(string)
		if dir = strings.TrimSpace(dir); dir == "" {
			dir = "."
		}
		loc := dir
		if filePattern, _ := one["file_pattern"].(string); strings.TrimSpace(filePattern) != "" {
			loc = strings.TrimRight(dir, "/") + "/" + strings.TrimSpace(filePattern)
		}
		if pattern, _ := one["pattern"].(string); strings.TrimSpace(pattern) != "" {
			loc += " [" + strings.TrimSpace(pattern) + "]"
		} else if recursive, ok := one["recursive"].(bool); ok && recursive {
			loc += " (recursive)"
		}
		labels = append(labels, loc)
	}
	return strings.Join(labels, ", ")
}

func joinStrings(value any) string {
	list, ok := value.([]any)
	if !ok {
		return ""
	}
	out := make([]string, 0, len(list))
	for _, one := range list {
		if str, ok := one.(string); ok && strings.TrimSpace(str) != "" {
			out = append(out, strings.TrimSpace(str))
		}
	}
	return strings.Join(out, ", ")
}

func todoProgress(value any) string {
	todos := asList(value)
	last, lastIdx := "", 0
	for i, one := range todos {
		content, _ := one["content"].(string)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		switch one["status"] {
		case "in_progress":
			if active, _ := one["active_form"].(string); strings.TrimSpace(active) != "" {
				content = strings.TrimSpace(active)
			}
			return fmt.Sprintf("%s [%d/%d]", content, i+1, len(todos))
		case "completed":
			last, lastIdx = content, i+1
		}
	}
	if last == "" {
		return ""
	}
	return fmt.Sprintf("%s [%d/%d]", last, lastIdx, len(todos))
}

func joinPaths(argMap map[string]any) string {
	paths := make([]string, 0, 4)
	for _, one := range asList(argMap["files"]) {
		if path, ok := one["path"].(string); ok && strings.TrimSpace(path) != "" {
			paths = append(paths, strings.TrimSpace(path))
		}
	}
	return strings.Join(paths, ", ")
}

func joinArgv(raw string) string {
	var params struct {
		Argv []string `json:"argv"`
	}
	if json.Unmarshal([]byte(raw), &params) != nil || len(params.Argv) == 0 {
		return ""
	}
	parts := make([]string, len(params.Argv))
	for i, one := range params.Argv {
		if one == "" || strings.ContainsAny(one, " \t\n\"'\\") {
			parts[i] = strconv.Quote(one)
			continue
		}
		parts[i] = one
	}
	return strings.Join(parts, " ")
}

func FormatEventFooter(duration, outputElapsed time.Duration, model, reasoning string, usage *go_llm_router.Usage) string {
	var parts []string
	if duration > 0 {
		elapsed := duration.Round(100 * time.Millisecond).String()
		if outputElapsed > 0 && usage != nil && usage.Output > 0 {
			elapsed += fmt.Sprintf("(%.1ft/s)", float64(usage.Output)/outputElapsed.Seconds())
		}
		parts = append(parts, elapsed)
	}

	if model = strings.TrimSpace(model); model != "" {
		if reasoning != "" {
			model += "/" + reasoning
		}
		parts = append(parts, model)
	}

	if in := agenttypes.FormatInput(agenttypes.InputTotals(usage)); in != "" {
		parts = append(parts, fmt.Sprintf("↑ %s ↓ %s", in, go_pkg_utils.CompactNumber(usage.Output)))
	}
	return strings.Join(parts, "  ")
}

type QuotaSource struct {
	ID string
	Fn func(context.Context, go_llm_router.Config) (go_llm_router.UsageRemaining, error)
}

var QuotaSources = []QuotaSource{
	{"codex", openaicodex.Usage},
	{"grok-oauth", grokoauth.Usage},
	{"copilot", copilot.Usage},
	{"openrouter", openrouter.Usage},
	{"deepseek", deepseek.Usage},
	{"claude-code", claudeCode.Usage},
}

func FormatQuota(remaining go_llm_router.UsageRemaining) string {
	if remaining.Balance != nil {
		return fmt.Sprintf("$%.2f", *remaining.Balance)
	}
	list := []string{}
	for _, value := range []*float64{remaining.FiveHour, remaining.Week, remaining.Total} {
		if value != nil {
			list = append(list, fmt.Sprintf("%.0f%%", *value))
		}
	}
	return strings.Join(list, "/")
}

func ModelQuota(ctx context.Context, model string) string {
	prefix, _, ok := strings.Cut(strings.TrimSpace(model), "@")
	if !ok {
		return ""
	}
	i := slices.IndexFunc(QuotaSources, func(source QuotaSource) bool {
		return source.ID == prefix
	})
	if i < 0 {
		return ""
	}
	source := QuotaSources[i]

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cfg, err := agentkeychain.Config(ctx, source.ID)
	if err != nil {
		return ""
	}
	remaining, err := source.Fn(ctx, cfg)
	if err != nil {
		return ""
	}
	return FormatQuota(remaining)
}

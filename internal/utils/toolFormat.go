package utils

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

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
	"public_api_list":       "API",
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
	"ask_user":              "Ask",
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
	"subagents":  {"list": "Subagents", "invoke": "Subagent"},
	"find_files": {"list": "Files", "glob": "Search", "search": "Search"},
	"edit_file":  {"write": "Write", "patch": "Edit", "remove": "Remove", "restore": "Restore"},
	"schedules":  {"write": "Write", "patch": "Edit", "remove": "Remove", "list": "Schedules"},
	"edit_skill": {"write": "Write", "patch": "Edit", "remove": "Remove"},
	"edit_tool":  {"write": "Write", "patch": "Edit", "remove": "Remove"},
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
					return clipValue(strings.TrimSpace(str))
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

func FormatTraceLine(ev agenttypes.Event) string {
	switch ev.Type {
	case agenttypes.EventReasoning, agenttypes.EventUserInjected, agenttypes.EventDaemonLog:
		return strings.TrimSpace(ev.Text)

	case agenttypes.EventToolCall:
		label := strings.Join(strings.Fields(FormatToolEvent(ev.ToolName, ev.ToolArgs)), " ")
		if label == "" {
			return ""
		}
		return "⏵ `" + label + "`"

	case agenttypes.EventToolSkipped:
		return "⏵ skipped `" + ev.ToolName + "`"

	case agenttypes.EventCompact:
		return "⏵ compacted history"

	case agenttypes.EventExecError:
		return "⚠ " + ev.ToolName + " — " + cmp.Or(ev.Text, "unknown error")

	case agenttypes.EventError:
		return "⚠ " + cmp.Or(ev.Text, "unknown error")
	}
	return ""
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

	case "public_api_list":
		return arg("category")

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

	case "ask_user":
		return askDetails(argMap["questions"])
	}

	if len(argMap) == 0 {
		return ""
	}
	if value := arg(primaryArgKeys...); value != "" {
		return value
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(clipArgs(argMap)); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

const maxEventValueRunes = 256

func clipValue(str string) string {
	runes := []rune(str)
	if len(runes) <= maxEventValueRunes {
		return str
	}
	return string(runes[:maxEventValueRunes]) + "..."
}

func clipArgs(value any) any {
	switch v := value.(type) {
	case string:
		return clipValue(v)
	case []any:
		list := make([]any, len(v))
		for i, one := range v {
			list[i] = clipArgs(one)
		}
		return list
	case map[string]any:
		dic := make(map[string]any, len(v))
		for key, one := range v {
			dic[key] = clipArgs(one)
		}
		return dic
	}
	return value
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
		if dir = clipValue(strings.TrimSpace(dir)); dir == "" {
			dir = "."
		}
		loc := dir
		if filePattern, _ := one["file_pattern"].(string); strings.TrimSpace(filePattern) != "" {
			loc = strings.TrimRight(dir, "/") + "/" + clipValue(strings.TrimSpace(filePattern))
		}
		if pattern, _ := one["pattern"].(string); strings.TrimSpace(pattern) != "" {
			loc += " [" + clipValue(strings.TrimSpace(pattern)) + "]"
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
			out = append(out, clipValue(strings.TrimSpace(str)))
		}
	}
	return strings.Join(out, ", ")
}

const maxAskDetailRunes = 128

func askDetails(value any) string {
	questions := asList(value)
	if len(questions) == 0 {
		return ""
	}
	detail, _ := questions[0]["detail"].(string)
	if strings.TrimSpace(detail) == "" {
		detail, _ = questions[0]["question"].(string)
	}
	detail, _, _ = strings.Cut(strings.TrimSpace(detail), "\n")
	if runes := []rune(detail); len(runes) > maxAskDetailRunes {
		detail = string(runes[:maxAskDetailRunes])
	}
	return detail
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
			return fmt.Sprintf("%s [%d/%d]", clipValue(content), i+1, len(todos))
		case "completed":
			last, lastIdx = content, i+1
		}
	}
	if last == "" {
		return ""
	}
	return fmt.Sprintf("%s [%d/%d]", clipValue(last), lastIdx, len(todos))
}

func joinPaths(argMap map[string]any) string {
	paths := make([]string, 0, 4)
	for _, one := range asList(argMap["files"]) {
		if path, ok := one["path"].(string); ok && strings.TrimSpace(path) != "" {
			paths = append(paths, clipValue(strings.TrimSpace(path)))
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
	if script, ok := shellScript(params.Argv); ok {
		return clipValue(script)
	}
	parts := make([]string, len(params.Argv))
	for i, one := range params.Argv {
		one = clipValue(one)
		if one == "" || strings.ContainsAny(one, " \t\n\"'\\") {
			parts[i] = strconv.Quote(one)
			continue
		}
		parts[i] = one
	}
	return strings.Join(parts, " ")
}

func shellScript(argv []string) (string, bool) {
	if len(argv) != 3 || argv[1] != "-c" {
		return "", false
	}
	switch filepath.Base(argv[0]) {
	case "sh", "bash", "zsh":
		return strings.TrimSpace(argv[2]), true
	}
	return "", false
}

var toolDisplayName = map[string]string{
	"search_web":           "Search",
	"find_tools":           "Find",
	"find_files":           "Find",
	"chat_history":         "Find",
	"file_history":         "Find",
	"error_history":        "Find",
	"list_chatbot":         "List",
	"read_files":           "Read",
	"write_result":         "Write",
	"edit_file":            "Edit",
	"run_command":          "Run",
	"run_skill":            "Run",
	"fetch_page":           "Fetch",
	"write_todo":           "Plan",
	"calculate":            "Calc",
	"send_to_chatbot":      "Send",
	"http_request":         "Send",
	"schedules":            "Cron",
	"subagents":            "Subagent",
	"background_command":   "Background",
	"background_subagents": "Background",
	"download_file":        "Download",
	"reasoning_guide":      "Guide",
}

func IsPlugTool(name string) bool {
	return strings.HasPrefix(name, "script_") ||
		strings.HasPrefix(name, "ext_")
}

func PlugToolBaseName(name string) string {
	for _, prefix := range []string{"script_", "ext_"} {
		if base, ok := strings.CutPrefix(name, prefix); ok {
			return base
		}
	}
	return name
}

func IsSubagentInvoke(name, raw string) bool {
	if name != "subagents" {
		return false
	}
	var dic struct {
		Mode string `json:"mode"`
		Task string `json:"task"`
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &dic)
	}
	if mode := strings.TrimSpace(dic.Mode); mode != "" {
		return mode == "invoke"
	}
	return strings.TrimSpace(dic.Task) != ""
}

func ToolName(name string) string {
	if d, ok := toolDisplayName[name]; ok {
		return d
	}
	if IsPlugTool(name) {
		var tag string
		switch {
		case strings.HasPrefix(name, "script_"):
			tag = "Script"
		case strings.HasPrefix(name, "ext_"):
			tag = "Extension"
		}
		return fmt.Sprintf("Plug Tool(%s %s)", tag, PlugToolBaseName(name))
	}
	return name
}

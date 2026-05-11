package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func FormatTool(name, raw string) string {
	if raw == "" {
		return ""
	}

	var argMap map[string]any
	if err := json.Unmarshal([]byte(raw), &argMap); err != nil {
		return raw
	}
	if len(argMap) == 0 {
		return ""
	}
	arg := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := argMap[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		return ""
	}

	switch name {
	case "invoke_subagent":
		val := arg("name", "session_id")
		if val == "" {
			val = "subagent"
		}
		if model := arg("model"); model != "" {
			val = fmt.Sprintf("%s (%s)", val, model)
		}

		task := arg("task")
		if task == "" {
			return val
		}
		return fmt.Sprintf("%s: %s", val, strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(task))

	case "activate_skill":
		if s := arg("skill", "name"); s != "" {
			return s
		}

	case "list_files":
		val := arg("dir", "path")
		if val == "" {
			break
		}
		if recursive, ok := argMap["recursive"].(bool); ok && recursive {
			return val + " (recursive)"
		}
		return val

	case "read_file", "write_file", "patch_file", "glob_files", "read_image", "save_page_to_file":
		if val := arg("path", "pattern", "save_to"); val != "" {
			return val
		}

	case "update_page":
		return ""

	case "search_web", "fetch_google_rss":
		if val := arg("query", "keyword"); val != "" {
			if timeRange := arg("time_range", "time"); timeRange != "" {
				return fmt.Sprintf("%s (%s)", val, timeRange)
			}
			return val
		}

	case "fetch_yahoo_finance":
		if val := arg("symbol"); val != "" {
			if timeRange := arg("time_range"); timeRange != "" {
				return fmt.Sprintf("%s (%s)", val, timeRange)
			}
			return val
		}

	case "fetch_page", "fetch_youtube_transcript":
		if val := arg("link", "url"); val != "" {
			return val
		}

	case "calculate":
		if val := arg("expression"); val != "" {
			return val
		}

	case "remember_error":
		if val := arg("symptom", "cause", "action"); val != "" {
			return val
		}

	case "search_error_memory", "search_conversation_history":
		if val := arg("keyword", "query"); val != "" {
			return val
		}

	case "run_command":
		var p struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(raw), &p); err != nil || len(p.Argv) == 0 {
			return raw
		}

		parts := make([]string, len(p.Argv))
		for i, arg := range p.Argv {
			if arg == "" || strings.ContainsAny(arg, " \t\n\"'\\") {
				parts[i] = strconv.Quote(arg)
			} else {
				parts[i] = arg
			}
		}
		return strings.Join(parts, " ")
	}
	return raw
}

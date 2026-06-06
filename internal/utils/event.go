package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

func FormatToolEvent(name, raw string) string {
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
		for _, key := range keys {
			if aryVal, ok := argMap[key]; ok {
				if str, ok := aryVal.(string); ok && strings.TrimSpace(str) != "" {
					return str
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

	case "run_skill":
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

	case "read_file", "write_file", "patch_file", "glob_files":
		if val := arg("path", "pattern"); val != "" {
			return val
		}

	case "search_web", "search_google_news":
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

	case "fetch_page":
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

	case "search_error_history", "search_chat_history":
		if val := arg("keyword", "query"); val != "" {
			return val
		}

	case "add_schedule", "patch_schedule":
		skill := arg("skill_name")
		t := arg("time")
		if skill != "" && t != "" {
			return fmt.Sprintf("%s %s", t, skill)
		}
		if skill != "" {
			return skill
		}

	case "remove_schedule":
		if skill := arg("skill_name"); skill != "" {
			return skill
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

func FormatEventFooter(duration time.Duration, model string, usage *agentTypes.Usage) string {
	var parts []string
	if duration > 0 {
		parts = append(parts, duration.Round(100*time.Millisecond).String())
	}

	if model = strings.TrimSpace(model); model != "" {
		if _, after, ok := strings.Cut(model, "@"); ok {
			model = after
		}
		parts = append(parts, model)
	}

	if usage != nil && (usage.Input > 0 || usage.Output > 0) {
		parts = append(parts, fmt.Sprintf("↑%s ↓%s", go_pkg_utils.CompactNumber(usage.Input), go_pkg_utils.CompactNumber(usage.Output)))
	}
	return strings.Join(parts, " · ")
}

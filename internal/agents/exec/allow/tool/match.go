package allowTool

import (
	"encoding/json"
	"strings"
)

func Match(rules []ToolRule, toolName, toolArgs string) bool {
	if len(rules) == 0 {
		return false
	}
	canonical := canonicalToolArgs(toolName, toolArgs)
	for _, r := range rules {
		if !r.name.MatchString(toolName) {
			continue
		}
		if !r.hasArg {
			return true
		}
		if r.argGlob.MatchString(canonical) {
			return true
		}
	}
	return false
}

func canonicalToolArgs(toolName, rawArgs string) string {
	rawArgs = strings.TrimSpace(rawArgs)
	if rawArgs == "" {
		return ""
	}
	switch toolName {
	case "run_command":
		var p struct {
			Argv []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(rawArgs), &p); err == nil && len(p.Argv) > 0 {
			return strings.Join(p.Argv, " ")
		}
	}
	var generic map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &generic); err == nil {
		for _, key := range []string{"path", "url", "file", "target", "command"} {
			if v, ok := generic[key].(string); ok && v != "" {
				return v
			}
		}
	}
	return rawArgs
}

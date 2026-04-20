package external

import "strings"

var prefixToAgent = map[string]string{
	"claude":  "claude",
	"codex":   "codex",
	"gh":      "copilot",
	"copilot": "copilot",
}

func MatchExternal(input string) (agent, effective string) {
	trimmed := strings.TrimLeft(input, " \t\r\n")
	if !strings.HasPrefix(trimmed, "/") {
		return "", input
	}
	rest := trimmed[1:]
	token := rest
	tail := ""
	if idx := strings.IndexAny(rest, " \t\r\n"); idx >= 0 {
		token = rest[:idx]
		tail = strings.TrimLeft(rest[idx:], " \t\r\n")
	}
	if token == "" {
		return "", input
	}
	resolved, ok := prefixToAgent[token]
	if !ok {
		return "", input
	}
	return resolved, tail
}

package external

import "strings"

var prefixToAgent = map[string]struct {
	agent    string
	readOnly bool
}{
	"claude":       {"claude", true},
	"claude-allow": {"claude", false},
	"codex":        {"codex", true},
	"codex-allow":  {"codex", false},
	"gh":           {"copilot", true},
	"copilot":      {"copilot", true},
	"gemini":       {"gemini", true},
	"gemini-allow": {"gemini", false},
}

func MatchExternal(input string) (agent, effective string, readOnly bool) {
	trimmed := strings.TrimLeft(input, " \t\r\n")
	if !strings.HasPrefix(trimmed, "/") {
		return "", input, true
	}
	rest := trimmed[1:]
	token := rest
	tail := ""
	if idx := strings.IndexAny(rest, " \t\r\n"); idx >= 0 {
		token = rest[:idx]
		tail = strings.TrimLeft(rest[idx:], " \t\r\n")
	}
	if token == "" {
		return "", input, true
	}
	resolved, ok := prefixToAgent[token]
	if !ok {
		return "", input, true
	}
	return resolved.agent, tail, resolved.readOnly
}

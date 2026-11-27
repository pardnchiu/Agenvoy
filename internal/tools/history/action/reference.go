package actionHistory

import (
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
)

var referenceTool = map[string]bool{
	"fetch_page":     true,
	"search_web":     true,
	"subagents":      true,
	"http_request":   true,
	"generate_image": true,
	"download_file":  true,
}

func isReference(name string) bool {
	if referenceTool[name] {
		return true
	}
	return strings.HasPrefix(name, "mcp__") ||
		strings.HasPrefix(name, configs.SCRIPT_TOOL_PREFIX) ||
		strings.HasPrefix(name, "ext_")
}

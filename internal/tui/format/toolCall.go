package format

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ToolCalls(data []byte, width int) string {
	var entries []struct {
		Role       string `json:"role"`
		Content    string `json:"content"`
		ToolCallID string `json:"tool_call_id"`
	}
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
		return ""
	}
	divider := strings.Repeat("─", width/2)
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if e.ToolCallID != "" {
			sb.WriteString(e.ToolCallID + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("[%s]\n", strings.ToUpper(e.Role)))
		}
		sb.WriteString(divider + "\n")
		sb.WriteString(e.Content)
		if !strings.HasSuffix(e.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(divider)
	}
	return sb.String()
}

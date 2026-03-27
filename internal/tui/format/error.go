package format

import (
	"encoding/json"
	"fmt"
	"strings"
)

func Error(data []byte, width int) string {
	var entries []struct {
		ID        string   `json:"id"`
		Timestamp int64    `json:"timestamp"`
		ToolName  string   `json:"tool_name"`
		Keywords  []string `json:"keywords"`
		Symptom   string   `json:"symptom"`
		Cause     string   `json:"cause"`
		Action    string   `json:"action"`
		Outcome   string   `json:"outcome"`
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
		sb.WriteString(e.ID + "\n")
		sb.WriteString(divider + "\n")
		if e.ToolName != "" {
			sb.WriteString(fmt.Sprintf("Tool     : %s\n", e.ToolName))
		}
		if e.Timestamp != 0 {
			sb.WriteString(fmt.Sprintf("Time     : %d\n", e.Timestamp))
		}
		if len(e.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("Keywords : %s\n", strings.Join(e.Keywords, ", ")))
		}
		if e.Symptom != "" {
			sb.WriteString(fmt.Sprintf("Symptom  : %s\n", e.Symptom))
		}
		if e.Cause != "" {
			sb.WriteString(fmt.Sprintf("Cause    : %s\n", e.Cause))
		}
		if e.Action != "" {
			sb.WriteString(fmt.Sprintf("Action   : %s\n", e.Action))
		}
		if e.Outcome != "" {
			sb.WriteString(fmt.Sprintf("Outcome  : %s\n", e.Outcome))
		}
		sb.WriteString(divider)
	}
	return sb.String()
}

package format

import (
	"encoding/json"
	"fmt"
	"strings"
)

func History(data []byte, width int) string {
	var entries []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
		return ""
	}
	divider := strings.Repeat("─", width/2)
	roleColor := map[string]string{
		"user":      "cyan",
		"assistant": "green",
		"tool":      "yellow",
	}
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		content := e.Content
		if after, ok := strings.CutPrefix(content, "---\n"); ok {
			if _, rest, found := strings.Cut(after, "\n---\n"); found {
				content = rest
			}
		}
		color := roleColor[e.Role]
		if color == "" {
			color = "white"
		}
		sb.WriteString(fmt.Sprintf("[%s::b]%s[-:-:-]\n", color, strings.ToUpper(e.Role)))
		sb.WriteString(fmt.Sprintf("[%s]%s[-]\n", color, divider))
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[%s]%s[-]", color, divider))
	}
	return sb.String()
}
